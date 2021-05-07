package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	discordhook "github.com/post04/webhook-utility"
)

var (
	// DB is the SQL database
	DB     *sql.DB
	config *Config
)

// Config is our config for config.json
type Config struct {
	ConsumerKey    string `json:"consumerKey"`
	ComsumerSecret string `json:"consumerSecret"`
	Bearer         string `json:"bearer"`
	AccessToken    string `json:"accessToken"`
	AccessSecret   string `json:"accessSecret"`
	WebhookURL     string `json:"webhookURL"`
}

type twitterResponse struct {
	Data struct {
		Text string `json:"text"`
	} `json:"data"`
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func (t *twitterResponse) getPos(str string) int {
	for i, part := range strings.Split(t.Data.Text, " ") {
		if part == str {
			return i
		}
	}
	return 0
}

func (t *twitterResponse) addBan() {
	sqlStmt := `INSERT INTO bans(profile, reports, timeStamp, name) VALUES (?, ?, ?, ?)`
	statement, err := DB.Prepare(sqlStmt)
	if err != nil {
		fmt.Println(err)
		return
	}
	pos := t.getPos("was")
	name := strings.Join(strings.Split(t.Data.Text, " ")[:pos], " ")
	reports := 0
	timeStamp := makeTimestamp()
	_, err = statement.Exec(strings.Split(t.Data.Text, " ")[len(strings.Split(t.Data.Text, " "))-1], reports, timeStamp, name)
	if err != nil {
		fmt.Println(err)
	}
}

func (t *twitterResponse) getLinkFromTco() {
	transport := http.Transport{}
	req, err := http.NewRequest("GET", strings.Split(t.Data.Text, " ")[len(strings.Split(t.Data.Text, " "))-1], nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	t.Data.Text = strings.ReplaceAll(t.Data.Text, strings.Split(t.Data.Text, " ")[len(strings.Split(t.Data.Text, " "))-1], resp.Header.Get("location"))
}

func (t *twitterResponse) sendToDiscord() {
	t.Data.Text = strings.ReplaceAll(t.Data.Text, "@here", "\\@here")
	discordhook.Send(config.WebhookURL, &discordhook.WebhookPayload{
		Content: strings.ReplaceAll(t.Data.Text, "@everyone", "\\@everyone"),
	})
}

func initSQL() {
	sqlDB, err := sql.Open("sqlite3", "./database.db")
	if err != nil {
		panic(err)
	}
	DB = sqlDB
	statement := `CREATE TABLE IF NOT EXISTS bans (
	"profile" TEXT,
	"reports" interger,
	"timeStamp" TEXT,
	"name" TEXT
	);`
	execute, err := DB.Prepare(statement)
	if err != nil {
		panic(err)
	}
	_, err = execute.Exec()
	if err != nil {
		panic(err)
	}
}

func initConfig() {
	config = &Config{}
	f, err := os.ReadFile("config.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(f, &config)
	if err != nil {
		panic(err)
	}
}

func main() {
	initSQL()
	initConfig()
	req, err := http.NewRequest("GET", "https://api.twitter.com/2/tweets/search/stream", nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+config.Bearer)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	d := json.NewDecoder(resp.Body)
	for {
		var obj twitterResponse
		err := d.Decode(&obj)
		if err != nil {
			panic(err)
		}
		obj.getLinkFromTco()
		obj.sendToDiscord()
		obj.addBan()
	}
}
