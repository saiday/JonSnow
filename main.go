package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/clbanning/mxj"
	"github.com/lib/pq"
	"gopkg.in/yaml.v2"
)

type Config struct {
	GooglePlayAppId    string `yaml:"google_play_app_id"`
	AppStoreAppId      string `yaml:"app_store_app_id"`
	ReviewCount        int    `yaml:"review_count"`
	BotName            string `yaml:"bot_name"`
	IconEmoji          string `yaml:"icon_emoji"`
	WebHookUri         string `yaml:"web_hook_uri"`
	GooglePlayLocation string `yaml:"google_play_location"`
	AppStoreLocation   string `yaml:"app_store_location"`
	AppStoreURI        string
}

type Review struct {
	Id        int
	Store     string
	Author    string
	Title     string
	Message   string
	Rate      string
	UpdatedAt time.Time `meddler:"updated_at,localtime"`
	Permalink string
	Color     string
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
	AuthorLink string                 `json:"author_link"`
	Title      string                 `json:"title"`
	TitleLink  string                 `json:"title_link"`
	Text       string                 `json:"text"`
	Fallback   string                 `json:"fallback"`
	Color      string                 `json:"color"`
	AuthorName string                 `json:"author_name"`
	Footer     string                 `json:"footer"`
	Fields     []SlackAttachmentField `json:"fields"`
}

type SlackAttachmentField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

const (
	TABLE_NAME                  = "review"
	GOOGLE_PLAY_BASE_URI        = "https://play.google.com/store/getreviews"
	APP_STORE_BASE_URI          = "https://itunes.apple.com"
	REVIEW_CLASS_NAME           = ".single-review"
	AUTHOR_NAME_CLASS_NAME      = ".review-info span.author-name"
	REVIEW_DATE_CLASS_NAME      = ".review-info .review-date"
	REVIEW_TITLE_CLASS_NAME     = ".review-body .review-title"
	REVIEW_MESSAGE_CLASS_NAME   = ".review-body"
	REVIEW_LINK_CLASS_NAME      = ".review-link"
	REVIEW_RATE_CLASS_NAME      = ".review-info-star-rating .current-rating"
	RATING_EMOJI                = ":star:"
	RATING_EMOJI_2              = ":star2:"
	MAX_REVIEW_NUM              = 40
	REVIEW_PERMALINK_CLASS_NAME = ".review-info .reviews-permalink"
)

var (
	dbh        *DBH
	configFile = flag.String("c", "./config.yml", "config file")
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

	if config.ReviewCount > MAX_REVIEW_NUM || config.ReviewCount < 1 {
		return config, fmt.Errorf("Please Set Num Between 1 and 40.")
	}

	url := os.Getenv("DATABASE_URL")
	fmt.Println(url)
	connection, _ := pq.ParseURL(url)
	connection += " sslmode=disable"

	db, err := sql.Open("postgres", connection)
	if err != nil {
		return config, err
	}

	err = db.Ping()
	if err != nil {
		return config, err
	}

	dbh = &DBH{db}

	// override BotName if environment variable found
	botName := os.Getenv("JON_SNOW_BOT_NAME")
	if botName != "" {
		config.BotName = botName
	}

	// override AppId if environment variable found
	googlePlayAppId := os.Getenv("JON_SNOW_GOOGLE_PLAY_APP_ID")
	if googlePlayAppId != "" {
		config.GooglePlayAppId = googlePlayAppId
	}

	// override AppId if environment variable found
	appStoreAppId := os.Getenv("JON_SNOW_APP_STORE_APP_ID")
	if appStoreAppId != "" {
		config.AppStoreAppId = appStoreAppId
	}

	// override WebHookUri if environment variable found
	webHookUri := os.Getenv("JON_SNOW_SLACK_HOOK")
	if webHookUri != "" {
		config.WebHookUri = webHookUri
	}

	// override Location if environment variable found
	googlePlayLocation := os.Getenv("JON_SNOW_GOOGLE_PLAY_LOCATION")
	if googlePlayLocation != "" {
		config.GooglePlayLocation = googlePlayLocation
	}

	// override Location if environment variable found
	appStoreLocation := os.Getenv("JON_SNOW_APP_STORE_LOCATION")
	if appStoreLocation != "" {
		config.AppStoreLocation = appStoreLocation
	}

	if config.AppStoreAppId == "" && config.GooglePlayAppId == "" {
		return config, fmt.Errorf("At least one of Google Play or App Store app id is required.")
	}

	appStoreURI := ""
	if id := config.AppStoreAppId; id != "" {
		appStoreURI = fmt.Sprintf("%s/%s/app/id%s", APP_STORE_BASE_URI, config.AppStoreLocation, id)
		config.AppStoreURI = appStoreURI
	}

	ids := []string{appStoreURI}
	err = CheckStoreURLAvailable(ids)
	if err != nil {
		return config, err
	}

	return config, err
}

func ValidateStoreURI(uri string) error {
	res, err := http.Get(uri)
	if err == nil && res.StatusCode == http.StatusNotFound {
		err = fmt.Errorf("URI: %s is not exists", uri)
	}
	return err
}

func CheckStoreURLAvailable(uris []string) error {
	var err error = nil
	for _, uri := range uris {
		if target := uri; target != "" {
			targetValidateErr := ValidateStoreURI(target)
			if targetValidateErr != nil {
				if err != nil {
					if err != nil {
						err = fmt.Errorf("URI Error: %v, %v", err, targetValidateErr)
					} else {
						err = fmt.Errorf("URI Error: %v", targetValidateErr)
					}
				}
			}
		}
	}
	return err
}

func main() {
	flag.Parse()

	config, err := NewConfig(*configFile)
	if err != nil {
		log.Println(err)
		return
	}

	if config.GooglePlayAppId != "" {
		err = ProcessGooglePlayReviews(config)

		if err != nil {
			log.Println(err)
			return
		}
	}

	if config.AppStoreURI != "" {
		err = ProcessAppStoreReviews(config)

		if err != nil {
			log.Println(err)
			return
		}
	}

	log.Println("all done.")
}

func ProcessGooglePlayReviews(config Config) error {
	log.Println("Processing Android reviews ...")

	reviews, err := GetGooglePlayReviews(config, GOOGLE_PLAY_BASE_URI, config.GooglePlayAppId, config.GooglePlayLocation)
	if err != nil {
		return err
	}

	reviews, err = SaveReviews(reviews)
	if err != nil {
		return err
	}

	err = PostReview(config, reviews)
	if err != nil {
		return err
	}

	log.Println("Google Play reviews process finished")

	return nil
}

func ProcessAppStoreReviews(config Config) error {
	log.Println("Processing App Store reviews ...")

	uri := config.AppStoreURI

	reviews, err := GetAppStoreReviews(config, uri)
	if err != nil {
		return err
	}

	reviews, err = SaveReviews(reviews)
	if err != nil {
		return err
	}

	err = PostReview(config, reviews)
	if err != nil {
		return err
	}

	log.Println("App Store reviews process finished")

	return nil
}

func escapedBytesToString(b []byte) string {
	b = bytes.Replace(b, []byte("\\u003c"), []byte("<"), -1)
	b = bytes.Replace(b, []byte("\\u003e"), []byte(">"), -1)
	b = bytes.Replace(b, []byte("\\u0026"), []byte("&"), -1)
	b = bytes.Replace(b, []byte("\\u003d"), []byte("="), -1)
	b = bytes.Replace(b, []byte("\\\""), []byte("\""), -1)
	return string(b)
}

func GetGooglePlayReviews(config Config, uri string, id string, hl string) (Reviews, error) {
	log.Println(fmt.Sprintf("id: %s, hl: %s", id, hl))
	hc := http.Client{}

	form := url.Values{}
	form.Add("hl", hl)
	form.Add("id", id)
	form.Add("reviewType", "0")
	form.Add("pageNum", "0")
	form.Add("reviewSortOrder", "0")
	form.Add("xhr", "1")

	req, err := http.NewRequest("POST", uri, strings.NewReader(form.Encode()))
	req.PostForm = form
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := hc.Do(req)

	defer resp.Body.Close()

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	bodyString := escapedBytesToString(bodyBytes)
	firstSpace := strings.Index(bodyString, " ")
	lastSpace := strings.LastIndex(bodyString, " ")
	content := html.UnescapeString(bodyString[firstSpace+1 : lastSpace])

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))

	if err != nil {
		return nil, err
	}

	reviews := Reviews{}

	doc.Find(REVIEW_CLASS_NAME).Each(func(i int, s *goquery.Selection) {
		authorNode := s.Find(AUTHOR_NAME_CLASS_NAME)
		authorName := authorNode.Text()
		dateNode := s.Find(REVIEW_DATE_CLASS_NAME)

		var timeForm string
		if config.GooglePlayLocation == "zh_TW" {
			timeForm = "2006年1月2日"
		} else if config.GooglePlayLocation == "en" {
			timeForm = "January 2, 2006"
		}

		date, err := time.Parse(timeForm, dateNode.Text())
		if err != nil {
			log.Println(err)
			return
		}

		reviewPermalinkNode := s.Find(REVIEW_PERMALINK_CLASS_NAME)
		reviewPermalink, _ := reviewPermalinkNode.Attr("href")

		reviewTitle := s.Find(REVIEW_TITLE_CLASS_NAME).Text()
		if len(reviewTitle) == 0 {
			reviewTitle = "No title provided"
		}

		reviewMessage := s.Find(REVIEW_MESSAGE_CLASS_NAME).Text()
		reviewLink := s.Find(REVIEW_LINK_CLASS_NAME).Text()

		reviewMessage = strings.Split(reviewMessage, reviewLink)[0]

		reviewRateNode := s.Find(REVIEW_RATE_CLASS_NAME)
		rateMessage, _ := reviewRateNode.Attr("style")

		rate := parseGooglePlayRate(rateMessage)

		review := Review{
			Author:    authorName,
			Store:     "Google Play",
			Title:     reviewTitle,
			Message:   reviewMessage,
			Rate:      rate,
			UpdatedAt: date,
			Permalink: fmt.Sprintf("%s%s", GOOGLE_PLAY_BASE_URI, reviewPermalink),
		}

		reviews = append(reviews, review)
	})

	sort.Sort(reviews)

	return reviews, nil
}

func GetAppStoreReviews(config Config, uri string) (Reviews, error) {
	log.Println(uri)

	rssUri := fmt.Sprintf("%s/%s/rss/customerreviews/page=1/id=%s/sortBy=mostRecent/xml", APP_STORE_BASE_URI, config.AppStoreLocation, config.AppStoreAppId)
	log.Println(rssUri)
	response, err := http.Get(rssUri)
	reviews := Reviews{}
	if err != nil {
		return nil, err
	} else {
		defer response.Body.Close()
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		data, err := mxj.NewMapXml(contents)
		if err != nil {
			log.Fatal("parsing xml failed")
			return nil, err
		}

		entries, err := data.ValuesForPath("feed.entry")
		if err != nil {
			log.Fatal("get xml entry failed")
			return nil, err
		}
		for i, entry := range entries {
			if i == 0 {
				continue
				// TODO: what's this
			}
			rate, _ := strconv.Atoi(entry.(map[string]interface{})["rating"].(string))
			commonData := entry.(map[string]interface{})
			author := commonData["author"].(map[string]interface{})

			updatedAt, err := time.Parse(time.RFC3339, commonData["updated"].(string))
			if err != nil {
				log.Fatal("parse time failed")
				return nil, err
			}

			message := commonData["content"].([]interface{})[0].(map[string]interface{})["#text"].(string)

			review := Review{
				Author:    author["name"].(string),
				Store:     "App Store",
				Title:     commonData["title"].(string),
				Message:   message,
				Rate:      parseAppStoreRate(rate),
				UpdatedAt: updatedAt,
				Permalink: author["uri"].(string),
			}

			reviews = append(reviews, review)
		}
	}
	sort.Sort(reviews)
	return reviews, nil
}

func parseAppStoreRate(count int) string {
	rateMessage := ""
	if count < 5 {
		rateMessage = strings.Repeat(RATING_EMOJI, count)
	} else {
		rateMessage = strings.Repeat(RATING_EMOJI_2, count)
	}

	return rateMessage
}

func parseGooglePlayRate(message string) string {
	rateMessage := ""

	switch {
	case strings.Contains(message, "width: 20%"):
		rateMessage = strings.Repeat(RATING_EMOJI, 1)
	case strings.Contains(message, "width: 40%"):
		rateMessage = strings.Repeat(RATING_EMOJI, 2)
	case strings.Contains(message, "width: 60%"):
		rateMessage = strings.Repeat(RATING_EMOJI, 3)
	case strings.Contains(message, "width: 80%"):
		rateMessage = strings.Repeat(RATING_EMOJI, 4)
	case strings.Contains(message, "width: 100%"):
		rateMessage = strings.Repeat(RATING_EMOJI_2, 5)
	}

	return rateMessage
}

func SaveReviews(reviews Reviews) (Reviews, error) {
	postReviews := Reviews{}

	for _, review := range reviews {
		var id int
		row := dbh.QueryRow("SELECT id FROM review WHERE comment_uri = $1", review.Permalink)
		err := row.Scan(&id)

		if err != nil {
			if err.Error() != "sql: no rows in result set" {
				return postReviews, err
			}
		}

		if id == 0 { // not exist
			_, err := dbh.Exec("INSERT INTO review (author, store, comment_uri, updated_at) VALUES ($1, $2, $3, $4)",
				review.Author, review.Store, review.Permalink, review.UpdatedAt)
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
			Title: "Rating",
			Value: review.Rate,
			Short: true,
		})

		fields = append(fields, SlackAttachmentField{
			Title: "UpdatedAt",
			Value: review.UpdatedAt.Format("2006-01-02"),
			Short: true,
		})

		attachments = append(attachments, SlackAttachment{
			Title:      review.Title,
			TitleLink:  review.Permalink,
			AuthorName: review.Author,
			Text:       review.Message,
			Fallback:   review.Message + " " + review.Author,
			Color:      review.Color,
			Fields:     fields,
			Footer:     review.Store,
		})
	}

	messageText := reviews[0].Store + " Reviews:"
	slackPayload := SlackPayload{
		UserName:    config.BotName,
		IconEmoji:   config.IconEmoji,
		Text:        messageText,
		Attachments: attachments,
	}

	payload, err := json.Marshal(slackPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", config.WebHookUri, bytes.NewBuffer([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")

	if err != nil {
		return err
	}

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
