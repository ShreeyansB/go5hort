package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	valid "github.com/asaskevich/govalidator"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Structs

type webhookReqBody struct {
	Message struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Entities []Entity `json:"entities"`
	} `json:"message"`
}

type Entity struct {
	Type string `json:"type"`
}

type sendMessageReqBody struct {
	ChatID    int64  `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

type MongoDatastore struct {
	Client  *mongo.Client
	Context *context.Context
}

type AppHandler struct {
	Handler func(w http.ResponseWriter, r *http.Request, db *MongoDatastore)
	Db      *MongoDatastore
}

type User struct {
	ID int64 `bson:"id,omitempty"`
}

type Token struct {
	Token string `bson:"token,omitempty"`
}

type AuthToken struct {
	Token string `bson:"token,omitempty"`
	ID    int64  `bson:"id,omitempty"`
}

type Pair struct {
	Link        string `bson:"link,omitempty"`
	Destination string `bson:"destination,omitempty"`
}

func main() {
	// Mongo
	// client, err := mongo.NewClient(options.Client().ApplyURI(getMongoURI()))
	client, err := mongo.NewClient(options.Client().ApplyURI(goDotEnvVariable("MONGO_URI")))

	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	cancel()
	defer client.Disconnect(ctx)

	mongoDatastore := MongoDatastore{Client: client, Context: &ctx}
	// Serving
	mux := http.NewServeMux()
	wHookHandler := AppHandler{Handler: MyHandler, Db: &mongoDatastore}
	urlHandler := AppHandler{Handler: ShortHandler, Db: &mongoDatastore}

	mux.HandleFunc("/", indexHandler)
	mux.Handle("/bot", wHookHandler)
	mux.Handle("/go/", urlHandler)
	http.ListenAndServe(":"+os.Getenv("PORT"), mux)
}

// ServeHTTP allows your type to satisfy the http.Handler interface.
func (ah AppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ah.Handler(w, r, ah.Db)
}

// Handlers

func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./index.html")
}

func ShortHandler(w http.ResponseWriter, r *http.Request, db *MongoDatastore) {
	path := strings.Split(r.URL.Path, "/")
	link := path[len(path)-1]
	b, d := checkLink(link, db)
	if !b {
		if d == "home" {
			http.ServeFile(w, r, "./index.html")
			return
		}
		http.ServeFile(w, r, "./404.html")
		return
	}
	d = parseURL(d)
	http.Redirect(w, r, d, http.StatusFound)
}

func MyHandler(res http.ResponseWriter, req *http.Request, db *MongoDatastore) {

	body := &webhookReqBody{}
	if err := json.NewDecoder(req.Body).Decode(body); err != nil {
		fmt.Println(req.Body)
		fmt.Println("could not decode request body", err)
		return
	}
	fmt.Println(body)

	isC, command := checkCommand(*body)
	if isC {
		if command[0] == "/start" {
			startMessage(body.Message.Chat.ID)
			return
		} else if command[0] == "/auth" {
			authUser(db.Client, db.Context, command[1], body.Message.Chat.ID, body.Message.From.ID)
			return
		} else if command[0] == "/short" {
			registerLink(command[1], db, body.Message.Chat.ID, req.Host)
			return
		} else {
			return
		}
	}
	if !checkAuth(db.Client, db.Context, body.Message.From.ID) {
		userNotAuth(body.Message.Chat.ID)
		return
	}

	// sendMessage(body.Message.Chat.ID, "Yo.")
}

// DB Functions

func checkAuth(client *mongo.Client, c *context.Context, i int64) bool {
	// ctx := *c
	user := User{ID: i}
	collection := client.Database("urlshortener").Collection("tokens")
	data := collection.FindOne(context.TODO(), user)
	return data.Err() == nil
}

func authUser(client *mongo.Client, c *context.Context, t string, chatID int64, uID int64) {
	token := Token{Token: t}
	var fAuth AuthToken
	collection := client.Database("urlshortener").Collection("tokens")
	data := collection.FindOne(context.TODO(), token)
	err := data.Decode(&fAuth)
	if err != nil {
		fmt.Println(err)
		sendMessage(chatID, "Invalid Auth Token.")
		return
	}
	if fAuth.ID != 0 {
		sendMessage(chatID, "Invalid Auth Token.")
		return
	} else {
		collection.UpdateOne(context.TODO(), token,
			bson.M{
				"$set": bson.M{"id": uID},
			})
		sendMessage(chatID, "Auth Token Verified.")
		return
	}
}

func checkLink(l string, db *MongoDatastore) (bool, string) {
	if l == "" {
		return false, "home"
	}
	coll := db.Client.Database("urlshortener").Collection("links")
	pair := Pair{Link: l}
	fmt.Println(pair)
	res := coll.FindOne(context.TODO(), pair)
	if res.Err() != nil {
		fmt.Println("ERROR")
		fmt.Println(res.Err())
		return false, ""
	}
	var p Pair
	res.Decode(&p)
	return true, p.Destination
}

func registerLink(l string, db *MongoDatastore, chatID int64, u string) {
	u = replaceHTTP(u)
	i := valid.IsURL(l)
	if !i {
		sendMessage(chatID, "URL is invalid.")
		return
	}
	coll := db.Client.Database("urlshortener").Collection("links")
	pair1 := Pair{Destination: l}
	pair2 := Pair{Destination: "https://" + l}

	res1 := coll.FindOne(context.TODO(), pair1)
	res2 := coll.FindOne(context.TODO(), pair2)
	var p Pair
	if res1.Err() == nil {
		res1.Decode(&p)
		msg := "Link: [URL](" + u + "/go/" + p.Link + ")"
		sendMessage(chatID, msg)
		return
	}
	if res2.Err() == nil {
		res2.Decode(&p)
		msg := "Link: [URL](" + u + "/go/" + p.Link + ")"
		sendMessage(chatID, msg)
		return
	}
	found := false
	var t Pair
	for i := 0; i < 100; i++ {
		t = Pair{Link: randSeq(6)}
		res := coll.FindOne(context.TODO(), t)
		if res.Err() != nil {
			found = true
			r := Pair{Link: t.Link, Destination: l}
			_, e := coll.InsertOne(context.TODO(), r)
			fmt.Println(e)
			msg := "Link: [URL](" + u + "/go/" + r.Link + ")"
			sendMessage(chatID, msg)
			break
		}
	}
	if !found {
		sendMessage(chatID, "Could not find free URL.")
		return
	}
}

// Helper Functions

func Contains(slice []int, val int) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func sendMessage(chatID int64, m string) error {
	reqBody := &sendMessageReqBody{
		ChatID:    chatID,
		Text:      m,
		ParseMode: "Markdown",
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	res, err := http.Post("https://api.telegram.org/bot"+goDotEnvVariable("BOT_TOKEN")+"/sendMessage", "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return errors.New("unexpected status" + res.Status)
	}

	return nil
}

func startMessage(chatID int64) error {
	reqBody := &sendMessageReqBody{
		ChatID:    chatID,
		Text:      "This is a link shortener bot.\nUse /auth <TOKEN> to authorize this bot.\nUse /short <LINK> to request a short link.",
		ParseMode: "Markdown",
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	res, err := http.Post("https://api.telegram.org/bot"+goDotEnvVariable("BOT_TOKEN")+"/sendMessage", "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return errors.New("unexpected status" + res.Status)
	}

	return nil
}

func checkCommand(m webhookReqBody) (bool, []string) {
	if m.Message.Entities != nil && m.Message.Entities[0].Type == "bot_command" {
		st := strings.Split(m.Message.Text, " ")
		var arg string
		if len(st) > 1 {
			arg = st[1]
		} else {
			arg = ""
		}
		return true, []string{st[0], arg}
	}
	return false, []string{}
}

func userNotAuth(chatID int64) error {
	reqBody := &sendMessageReqBody{
		ChatID:    chatID,
		Text:      "User not authorized",
		ParseMode: "Markdown",
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	res, err := http.Post("https://api.telegram.org/bot"+goDotEnvVariable("BOT_TOKEN")+"/sendMessage", "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return errors.New("unexpected status" + res.Status)
	}

	return nil
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

func randSeq(n int) string {
	rand.Seed(time.Now().UTC().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func parseURL(u string) string {
	i := strings.Contains(u, "http")
	if i {
		return u
	} else {
		return "https://" + u
	}
}

func replaceHTTP(l string) string {
	// i := strings.Contains(l, "https")
	// if i {
	// 	return l
	// } else {
	// 	return strings.Replace(l, "http", "https", 1)
	// }
	return "https://" + l
}

func goDotEnvVariable(key string) string {

	// load .env file
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}
