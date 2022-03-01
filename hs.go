package main

import (
	"fmt"
	"github.com/RediSearch/redisearch-go/redisearch"
	"github.com/gocolly/colly/v2"
	"github.com/gomodule/redigo/redis"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type HS struct {
	r  redis.Conn
	rs *redisearch.Client
}

func (hs *HS) InitRedisAndRedisSearch(redisRawUrl, indexName string) {
	if len(strings.TrimSpace(redisRawUrl)) == 0 {
		redisRawUrl = "redis://127.0.0.1:55007"
	}
	conn, err := redis.DialURL(redisRawUrl)
	p := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return conn, err
		},
		MaxIdle:     3,                 // adjust to taste
		IdleTimeout: 240 * time.Second, // adjust to taste
	}
	hs.r = conn
	hs.rs = redisearch.NewClientFromPool(p, indexName)
}

func (hs *HS) CreateAndInitIndexDoc() {
	// Drop an existing index. If the index does not exist an error is returned
	_ = hs.rs.Drop()
	// Create a schema
	sc := redisearch.NewSchema(redisearch.DefaultOptions).
		AddField(redisearch.NewTextFieldOptions("title", redisearch.TextFieldOptions{Weight: 5.0, Sortable: true})).
		AddField(redisearch.NewTextField("content")).
		AddField(redisearch.NewTextField("link")).
		AddField(redisearch.NewTextField("publishTime"))

	// Create the index with the given schema
	definition := redisearch.NewIndexDefinition().AddPrefix("blog:")
	if err := hs.rs.CreateIndexWithIndexDefinition(sc, definition); err != nil {
		log.Fatal(err)
	}
	// Create a document with an id and given score
	var docs []redisearch.Document
	for i, article := range FetchAllArticles() {
		reply, err := hs.r.Do("HSET", fmt.Sprintf("blog:%d", i),
			"title", article.Title,
			"content", article.Content,
			"link", article.Link,
			"date", time.Now().Unix(),
		)
		if err != nil {
			log.Println("redis do err", err, reply)
		}
	}

	indexingOptions := redisearch.IndexingOptions{
		Language: "chinese",
		Replace:  true,
	}
	// Create the index with the given schema
	if err := hs.rs.IndexOptions(indexingOptions, docs...); err != nil {
		log.Fatal(err)
	}
	fmt.Println("index completed.")
}

func (hs *HS) Search(writer http.ResponseWriter, request *http.Request) {
	err := request.ParseForm()
	if err != nil {
		return
	}
	keyword := request.FormValue("keyword")
	pageSize, _ := strconv.Atoi(request.FormValue("pageSize"))
	pageNum, _ := strconv.Atoi(request.FormValue("pageNum"))
	if pageNum == 0 {
		pageNum = redisearch.DefaultNum
	}
	// Searching with limit and sorting
	docs, total, err := hs.rs.Search(redisearch.NewQuery(keyword).
		Limit(pageSize, pageNum).
		SetLanguage("chinese").
		SetReturnFields("title", "link"))
	for _, doc := range docs {
		fmt.Fprintln(writer, doc.Id, doc.Properties["title"], doc.Properties["link"], total)
	}
}

func FetchAllArticles() []Article {
	var articles []Article
	c := colly.NewCollector()
	c.OnHTML("a.page-number", func(e *colly.HTMLElement) {
		url := e.Request.AbsoluteURL(e.Attr("href"))
		if err := c.Visit(url); err != nil {
			log.Println(fmt.Sprintf("error:%s ,visit url:%s ", err, url))
		}
	})
	c.OnHTML("h2.post-title > a[rel='bookmark']", func(e *colly.HTMLElement) {
		url := e.Request.AbsoluteURL(e.Attr("href"))
		if err := c.Visit(e.Request.AbsoluteURL(e.Attr("href"))); err != nil {
			log.Println(fmt.Sprintf("error:%s ,visit url:%s ", err, url))
		}
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
	if err := c.Visit("https://1eveye.cn"); err != nil {
		log.Println("visit error", err)
	}
	return articles
}

type Article struct {
	Cover       string
	Link        string
	Title       string
	Tags        []string
	Categories  []string
	Content     string
	Author      string
	PublishTime time.Time
}

func (a *Article) String() string {
	return fmt.Sprintf("title: %s\nlink: %s\ncontent: %s\n", a.Title, a.Link, a.Content)
}
