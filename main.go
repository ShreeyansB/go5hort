package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Pair struct {
	Link        string `bson:"link,omitempty"`
	Destination string `bson:"destination,omitempty"`
}

func main() {
	// Mongo

	client, err := mongo.NewClient(options.Client().ApplyURI(getMongoURI()))
	if err != nil {
		log.Fatal(err)
	}
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	// Serving

	mux := defaultMux()

	// pathsToUrls := map[string]string{
	// 	"/go": "https://google.com",
	// }
	mapHandler := MapHandler(client, &ctx, mux)

	fmt.Println("Starting the server on :8080")
	http.ListenAndServe(":8080", mapHandler)
}

func defaultMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", index)
	return mux
}

func index(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./index.html")
}

func fetchDB(client *mongo.Client, c *context.Context) []Pair {
	ctx := *c
	collection := client.Database("urlshortener").Collection("links")
	data, derr := collection.Find(ctx, bson.D{})
	if derr != nil {
		panic(derr)
	}
	defer data.Close(ctx)
	var results []Pair
	if err := data.All(ctx, &results); err != nil {
		panic(err)
	}
	fmt.Println(results)
	return results
}
