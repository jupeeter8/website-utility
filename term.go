package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/charmbracelet/glamour"
	"golang.org/x/net/html"
)

type Rss struct {
	Channel *Channel `xml:"channel"`
}

type Channel struct {
	ItemList []Item `xml:"item"`
}

type Item struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
}

var rss Rss
var NegativeId = errors.New("getHtml: Negative id value recieved in request")

func getXML() {
	rss = Rss{}
	resp, err := http.Get(fmt.Sprintf("%s/feed", cfg.BaseUrl))
	if err != nil {
		fmt.Printf("getXML: failed to fetch rss feed %s", err)
		panic(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		fmt.Printf("getXML: failure in reading body of the response %s", err)
		panic(err)
	}

	if err := xml.Unmarshal(body, &rss); err != nil {
		fmt.Printf("getXML: failure in parsing XML body %s", err)
		panic(err)
	}

	sort.Slice(rss.Channel.ItemList, func(i, j int) bool {
		timeI, err1 := time.Parse(time.RFC1123, rss.Channel.ItemList[i].PubDate)
		timeJ, err2 := time.Parse(time.RFC1123, rss.Channel.ItemList[j].PubDate)

		if err1 != nil || err2 != nil {
			return false
		}
		return timeJ.After(timeI)

	})
}

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

func getHtml(id string, theme string) (string, error) {

	var idx int
	idx, err := strconv.Atoi(id)

	if err != nil {
		fmt.Printf("getHtml: error converting id to an int %s", err)
		idx = len(rss.Channel.ItemList) - 1
	}

	if idx < 0 {
		return "", NegativeId
	}

	// If requested page ID is greater than the total pages
	// set idx to most recent page
	if idx > len(rss.Channel.ItemList)-1 {
		idx = len(rss.Channel.ItemList) - 1
	}

	var item Item = rss.Channel.ItemList[idx]
	resp, err := http.Get(item.Link)
	if err != nil {
		return "", fmt.Errorf("getHtml: netowrk request failed on get %s %w", item.Link, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return "", fmt.Errorf("getHtml: failed to read response body %s %w", item.Link, err)
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
		return "", fmt.Errorf("getHtml: markdown conversion failed for %s %w", item.Title, err)
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
		fmt.Printf("renderMarkdown: An error occured during rendering markdown %s", err)
		out, _ := r.Render("# Error 500")
		return out
	}

	return out
}
