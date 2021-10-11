# go5hort
go5hort is a Telegram Bot written in Go for Shortening URLs.

## Commands
```
/start - Information
/auth <TOKEN> - Authorise User for Bot Use
/short <URL> - Create a Short URL
```

## Deploying

### Bot Setup 
Create a bot [@BotFather](t.me/botfather). Get the API Key and set it as:
```
BOT_TOKEN=14532169:JksmKMS975SAYoUrT0k3NdqiqNS84nSUD89e
```
in a new file called **.env** in root.

### DB Setup
Create a cluster on MongoDB Atlas and set up database as follows:
```
links (collection)
├──link
└──destination

tokens (collection)
├──token (assign your token)
└──id (tg user ID - initially empty)
```
Get the database URI with username and password and set it as:
```
MONGO_URI=mongodb+srv://user:pass@cluster.mongodb.net/bonk?retryWrites=true&w=majority
```
in **.env** in root.

Customize **index.html** and **404.html** as per your liking.

> Also use **go mod vendor** to download libraries in root folder for deployment. I used Heroku for deployment. Dont forget to set the environment variables for your preffered method of deployment. Eg: Set Config Vars in Heroku Dashboard.

