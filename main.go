package main

import (
	"fmt"
	"github.com/RediSearch/redisearch-go/redisearch"
	"github.com/go-redis/redis"
	"github.com/gocolly/colly/v2"
	"log"
	"net/http"
	"strconv"
	"time"
)

var c *redisearch.Client
var client *redis.Client

func init() {
	c = redisearch.NewClient("localhost:55007", "deveye")
	client = redis.NewClient(&redis.Options{
		Addr:     "localhost:55007",
		Password: "", // no password set
		DB:       0,  // use default DB
		PoolSize: 10, // 连接池的大小为 10
	})

	// Drop an existing index. If the index does not exist an error is returned
	c.Drop()
	// Create a schema
	sc := redisearch.NewSchema(redisearch.DefaultOptions).
		AddField(redisearch.NewTextFieldOptions("title", redisearch.TextFieldOptions{Weight: 5.0, Sortable: true})).
		AddField(redisearch.NewTextField("content")).
		AddField(redisearch.NewTextField("link")).
		AddField(redisearch.NewTextField("date"))

	// Create the index with the given schema
	definition := redisearch.NewIndexDefinition().AddPrefix("blog:")
	if err := c.CreateIndexWithIndexDefinition(sc, definition); err != nil {
		log.Fatal(err)
	}

	// Create a document with an id and given score
	var docs []redisearch.Document
	for i, article := range FetchAllArticles() {
		client.Do("HSET", fmt.Sprintf("blog:%d", i),
			"title", article.Title,
			"content", article.Content,
			"link", article.Link,
			"date", time.Now().Unix(),
		)
	}

	indexingOptions := redisearch.IndexingOptions{
		Language: "chinese",
		Replace:  true,
	}
	// Create the index with the given schema
	if err := c.IndexOptions(indexingOptions, docs...); err != nil {
		log.Fatal(err)
	}
	fmt.Println("index completed.")
}

func main() {
	http.HandleFunc("/search", Search)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func Search(writer http.ResponseWriter, request *http.Request) {
	err := request.ParseForm()
	if err != nil {
		return
	}
	keyword := request.FormValue("keyword")
	pageSize, _ := strconv.Atoi(request.FormValue("pageSize"))
	pageNum, _ := strconv.Atoi(request.FormValue("pageNum"))
	// Searching with limit and sorting
	docs, total, err := c.Search(redisearch.NewQuery(keyword).
		Limit(pageNum, pageSize).
		SetLanguage("chinese").
		SetReturnFields("title", "link"))
	for _, doc := range docs {
		fmt.Fprintln(writer, doc.Id, doc.Properties["title"], doc.Properties["link"], total)
	}
}

func FetchAllArticles() []Article {
	var articles []Article
	c := colly.NewCollector()
	//每页数据
	c.OnHTML("a.page-number", func(e *colly.HTMLElement) {
		c.Visit(e.Request.AbsoluteURL(e.Attr("href")))
	})
	c.OnHTML("h2.post-title > a[rel='bookmark']", func(e *colly.HTMLElement) {
		c.Visit(e.Request.AbsoluteURL(e.Attr("href")))
	})
	c.OnHTML("article", func(e *colly.HTMLElement) {
		title := e.DOM.Find("h1.post-title").Text()
		content := e.DOM.Find("div.post-content").Text()
		article := Article{
			Title:   title,
			Content: content,
			Link:    e.Request.URL.String(),
		}
		articles = append(articles, article)
	})
	c.Visit("https://deveye.cn")
	return articles
}

type Article struct {
	Title   string
	Link    string
	Content string
}

func (a *Article) String() string {
	return fmt.Sprintf("title: %s\nlink: %s\ncontent: %s\n", a.Title, a.Link, a.Content)
}
