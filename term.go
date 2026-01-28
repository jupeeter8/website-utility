package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/charmbracelet/glamour"
	"golang.org/x/net/html"
)

var NegativeId = errors.New("getHtml: Negative id value recieved in request")

func getPageList() (string, error) {

	// Generates a markdown list of all blog posts with thier indexes
	// as week number followed by title and pubtime
	var b strings.Builder

	_, err := b.WriteString("# The Blog\n")
	if err != nil {
		return "", fmt.Errorf("getPageList: failed to create markdown string header %w", err)
	}

	template := "- **_Week %d_** %s %s \n"

	for i, p := range rss.Channel.ItemList {
		pubTime, err := time.Parse(time.RFC1123, p.PubDate)

		if err != nil {
			return "", fmt.Errorf("getPageList: parsing of time failed for rss pubDates %w", err)
		}

		format := pubTime.Format("Mon 02 Jan 2006")
		fmt.Fprintf(&b, template, i, p.Title, format)
	}

	markdown := b.String()
	return markdown, nil
}

func parseHTML(body []byte) (string, error) {

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("parseHTML: error in parsing the html content %w", err)
	}

	// Append usage info on top of every blog post HTML
	strongNode := &html.Node{
		Type: html.ElementNode,
		Data: "strong",
	}
	strongNode.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: fmt.Sprintf("👉 Run curl %s/help for usage", cfg.NewsUrl),
	})

	pNode := &html.Node{
		Type: html.ElementNode,
		Data: "p",
	}
	pNode.AppendChild(strongNode)

	// Insert at the top of <body> or <html>
	var bodyNode *html.Node
	var findBody func(*html.Node)
	findBody = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			bodyNode = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findBody(c)
		}
	}
	findBody(doc)

	if bodyNode != nil {
		bodyNode.InsertBefore(pNode, bodyNode.FirstChild)
	} else {
		doc.FirstChild.InsertBefore(pNode, doc.FirstChild.FirstChild)
	}

	var sb strings.Builder
	if err := html.Render(&sb, doc); err != nil {
		return "", fmt.Errorf("parseHTML: error in building string from parsed HTML %w", err)
	}

	return sb.String(), nil
}

func getHTML(pageURL, pageTitle string) (string, error) {

	log.Printf("Making network request")

	resp, err := http.Get(pageURL)
	if err != nil {
		return "", fmt.Errorf("getHtml: netowrk request failed on get %s %w", pageURL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return "", fmt.Errorf("getHtml: failed to read response body %s %w", pageURL, err)
	}

	htmlData, err := parseHTML(body)
	if err != nil {
		return "", fmt.Errorf("getHTML: %w", err)
	}

	markdown, err := htmltomarkdown.ConvertString(
		htmlData,
		converter.WithDomain(cfg.BaseUrl),
	)
	if err != nil {
		return "", fmt.Errorf("getHtml: markdown conversion failed for %s %w", pageTitle, err)
	}
	return markdown, nil
}
func getRssPageData(idx int) (Item, int, error) {

	if idx < 0 {
		return Item{}, 0, NegativeId
	}

	if idx > len(rss.Channel.ItemList)-1 {
		// TODO: This will force updates everytime a id
		// greater than the total list size is passed
		// Need a mechanism to ignore repetated fetch
		// within a time frame
		log.Printf("Requested id: %d is larger than the rss content list, fetching rss data gain", idx)
		var updates = make(chan Rss)
		go updateRssList(updates)
		rss = <-updates
		sortFeed()

		if idx > len(rss.Channel.ItemList) - 1 {
			idx = len(rss.Channel.ItemList) - 1
		}
	}

	return rss.Channel.ItemList[idx], idx, nil
}

func getPageMarkdown(idx int, item Item) (string, error) {

	latest := false
	type result struct {
		md string
		err error
	}

	if markdown, isSet := getFromCache(idx);  isSet {
		log.Println("cache lookup successfull")
		return markdown, nil
	} else {
		log.Printf("getPageMarkdown: cache is not set for the id: %d\n", idx)
	}

	ch := make(chan result, 1)

	go func() {
		m, e := getHTML(item.Link, item.Title)
		ch <- result{m, e}
	}()

	res := <-ch
	if res.err == nil {
		if idx == len(rss.Channel.ItemList) - 1 {
			latest = true
		}
		storeInCache(idx, res.md, latest)
		return res.md, nil
	}
	return "", res.err
}

func genPageMarkdown(id string, theme string) (string, error) {

	var idx int
	idx, err := strconv.Atoi(id)

	if err != nil {
		log.Printf("genPageMarkdown: error converting id to an int %s\n", err)
		idx = len(rss.Channel.ItemList) - 1
	}

	item, idx, err := getRssPageData(idx)

	if err != nil {
		log.Printf("Failed to get rss page data for the specifed index\n")
		return "", err
	}

	markdown, err := getPageMarkdown(idx, item)

	if err != nil {
		log.Printf("genPageMarkdown: getPageMarkdown failed: %s\n", err)
	}
	
	return renderMarkdown(markdown, 120, theme), nil
}

func renderMarkdown(data string, wrap int, theme string) string {

	if wrap == -1 {
		wrap = 100
	}

	if theme != "light" {
		theme = "dark"
	}

	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle(theme),
		glamour.WithWordWrap(wrap),
	)

	out, err := r.Render(data)
	if err != nil {
		log.Printf("renderMarkdown: An error occured during rendering markdown %s", err)
		out, _ := r.Render("# Error 500")
		return out
	}

	return out
}
