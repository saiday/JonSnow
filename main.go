package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/lib/pq"
	"gopkg.in/yaml.v2"
)

type Config struct {
	AppId       string `yaml:"app_id"`
	ReviewCount int    `yaml:"review_count"`
	BotName     string `yaml:"bot_name"`
	IconEmoji   string `yaml:"icon_emoji"`
	MessageText string `yaml:"message_text"`
	WebHookUri  string `yaml:"web_hook_uri"`
	DbPath      string `yaml:"db_path"`
}

type Review struct {
	Id        int
	Author    string
	AuthorUri string `meddler:"author_uri"`
	Title     string
	Message   string
	Rate      string
	UpdatedAt time.Time `meddler:"updated_at,localtime"`
}

type Reviews []Review

type DBH struct {
	*sql.DB
}

type SlackPayload struct {
	Text        string            `json:"text"`
	UserName    string            `json:"username"`
	IconEmoji   string            `json:"icon_emoji"`
	Attachments []SlackAttachment `json:"attachments"`
}

type SlackAttachment struct {
	Title     string                 `json:"title"`
	TitleLink string                 `json:"title_link"`
	Text      string                 `json:"text"`
	Fallback  string                 `json:"fallback"`
	Fields    []SlackAttachmentField `json:"fields"`
}

type SlackAttachmentField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

const (
	TABLE_NAME                = "review"
	BASE_URI                  = "https://play.google.com"
	REVIEW_CLASS_NAME         = ".single-review"
	AUTHOR_NAME_CLASS_NAME    = ".review-info span.author-name a"
	REVIEW_DATE_CLASS_NAME    = ".review-info .review-date"
	REVIEW_TITLE_CLASS_NAME   = ".review-body .review-title"
	REVIEW_MESSAGE_CLASS_NAME = ".review-body"
	REVIEW_LINK_CLASS_NAME    = ".review-link"
	REVIEW_RATE_CLASS_NAME    = ".review-info-star-rating .current-rating"
	RAITING_EMOJI             = ":star:"
	MAX_REVIEW_NUM            = 40
)

var (
	dbh        *DBH
	configFile = flag.String("c", "../config.yml", "config file")
)

func GetDBH() *DBH {
	return dbh
}

func (dbh *DBH) LastInsertId(tableName string) int {
	row := dbh.QueryRow(`SELECT id FROM ` + tableName + ` ORDER BY id DESC LIMIT 1`)

	var id int
	err := row.Scan(&id)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return 0
		}
		log.Fatal(err)
	}

	return id
}

func NewConfig(path string) (config Config, err error) {
	config = Config{}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return config, err
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, err
	}

	if config.AppId == "" {
		return config, fmt.Errorf("Please Set Your Google Play App Id.")
	}

	if config.ReviewCount > MAX_REVIEW_NUM || config.ReviewCount < 1 {
		return config, fmt.Errorf("Please Set Num Between 1 and 40.")
	}

	url := os.Getenv("DATABASE_URL")
	connection, _ := pq.ParseURL(url)
	connection += " sslmode=require"

	db, err := sql.Open("postgres", connection)
	if err != nil {
		return config, err
	}

	err = db.Ping()
	if err != nil {
		return config, err
	}

	dbh = &DBH{db}

	uri := fmt.Sprintf("%s/store/apps/details?id=%s", BASE_URI, config.AppId)

	res, err := http.Get(uri)
	if err != nil {
		return config, err
	}

	if res.StatusCode == http.StatusNotFound {
		return config, fmt.Errorf("AppID: %s is not exists", config.AppId)
	}

	return config, err
}

func main() {
	flag.Parse()

	config, err := NewConfig(*configFile)
	if err != nil {
		log.Println(err)
		return
	}

	reviews, err := GetReview(config)
	if err != nil {
		log.Println(err)
		return
	}

	reviews, err = SaveReviews(reviews)
	if err != nil {
		log.Println(err)
		return
	}

	err = PostReview(config, reviews)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("done")
}

func GetReview(config Config) (Reviews, error) {
	uri := fmt.Sprintf("%s/store/apps/details?id=%s&hl=zh-tw", BASE_URI, config.AppId)
	log.Println(uri)
	doc, err := goquery.NewDocument(uri)

	if err != nil {
		return nil, err
	}

	reviews := Reviews{}

	doc.Find(REVIEW_CLASS_NAME).Each(func(i int, s *goquery.Selection) {
		authorNode := s.Find(AUTHOR_NAME_CLASS_NAME)

		authorName := authorNode.Text()
		authorUri, _ := authorNode.Attr("href")

		dateNode := s.Find(REVIEW_DATE_CLASS_NAME)

		const timeForm = "2006年1月2日"
		date, err := time.Parse(timeForm, dateNode.Text())
		if err != nil {
			log.Println(err)
			return
		}

		reviewTitle := s.Find(REVIEW_TITLE_CLASS_NAME).Text()
		reviewMessage := s.Find(REVIEW_MESSAGE_CLASS_NAME).Text()
		reviewLink := s.Find(REVIEW_LINK_CLASS_NAME).Text()

		reviewMessage = strings.Split(reviewMessage, reviewLink)[0]

		reviewRateNode := s.Find(REVIEW_RATE_CLASS_NAME)
		rateMessage, _ := reviewRateNode.Attr("style")

		rate := parseRate(rateMessage)

		review := Review{
			Author:    authorName,
			AuthorUri: authorUri,
			Title:     reviewTitle,
			Message:   reviewMessage,
			Rate:      rate,
			UpdatedAt: date,
		}

		reviews = append(reviews, review)
	})

	sort.Sort(reviews)

	return reviews, nil
}

func parseRate(message string) string {
	rate := ""

	switch {
	case strings.Contains(message, "width: 20%"):
		rate = strings.Repeat(RAITING_EMOJI, 1)
	case strings.Contains(message, "width: 40%"):
		rate = strings.Repeat(RAITING_EMOJI, 2)
	case strings.Contains(message, "width: 60%"):
		rate = strings.Repeat(RAITING_EMOJI, 3)
	case strings.Contains(message, "width: 80%"):
		rate = strings.Repeat(RAITING_EMOJI, 4)
	case strings.Contains(message, "width: 100%"):
		rate = strings.Repeat(RAITING_EMOJI, 5)
	}

	return rate
}

func SaveReviews(reviews Reviews) (Reviews, error) {
	postReviews := Reviews{}

	for _, review := range reviews {
		var id int
		row := dbh.QueryRow("SELECT id FROM review WHERE author_uri = $1", review.AuthorUri)
		err := row.Scan(&id)

		if err != nil {
			if err.Error() != "sql: no rows in result set" {
				return postReviews, err
			}
		}

		if id == 0 {
			_, err := dbh.Exec("INSERT INTO review (author, author_uri, updated_at) VALUES ($1, $2, $3)",
				review.Author, review.AuthorUri, review.UpdatedAt)
			if err != nil {
				return postReviews, err
			}
			postReviews = append(postReviews, review)
		}
	}

	return postReviews, nil
}

func PostReview(config Config, reviews Reviews) error {
	attachments := []SlackAttachment{}

	if 1 > len(reviews) {
		return nil
	}

	for i, review := range reviews {
		if i >= config.ReviewCount {
			break
		}

		fields := []SlackAttachmentField{}

		fields = append(fields, SlackAttachmentField{
			Title: "Raiting",
			Value: review.Rate,
			Short: true,
		})

		fields = append(fields, SlackAttachmentField{
			Title: "UpdatedAt",
			Value: review.UpdatedAt.Format("2006-01-02"),
			Short: true,
		})

		attachments = append(attachments, SlackAttachment{
			Title:     review.Author,
			TitleLink: fmt.Sprintf("%s%s", BASE_URI, review.AuthorUri),
			Text:      review.Message,
			Fallback:  review.Message + " " + review.AuthorUri,
			Fields:    fields,
		})
	}

	slackPayload := SlackPayload{
		UserName:    config.BotName,
		IconEmoji:   config.IconEmoji,
		Text:        config.MessageText,
		Attachments: attachments,
	}
	payload, err := json.Marshal(slackPayload)
	if err != nil {
		return err
	}

	req, _ := http.NewRequest("POST", config.WebHookUri, bytes.NewBuffer([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")

	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return nil
}

func (r Reviews) Len() int {
	return len(r)
}

func (r Reviews) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r Reviews) Less(i, j int) bool {
	return r[i].UpdatedAt.Unix() > r[j].UpdatedAt.Unix()
}
