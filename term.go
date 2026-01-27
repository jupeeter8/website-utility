package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
func genPageMarkdown(id string, theme string) (string, error) {

	var markdown string
	var idx int

	idx, err := strconv.Atoi(id)

	if err != nil {
		log.Printf("getHtml: error converting id to an int %s\n", err)
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
	_, prs := cache[item.Title]

	storefile := func(filename string) {
		file, err := os.Create(filename)
		if err != nil {
			log.Printf("Error creating file: %s", err)
		} else {
			defer file.Close()
			_, err = file.WriteString(markdown)

			if err != nil {
				log.Printf("Error writing to the file: %s", err)
			}
		}
	}
	filename := fmt.Sprintf("%s.txt", item.Title)
	if !prs {
		log.Printf("Page \"%s\" not present in the cache", item.Title)
		cache[item.Title] = filename
	}

	if _, err := os.Stat(filename); err != nil {

		markdown, err = getHTML(item.Link, item.Title)
		if err != nil {
			return "", fmt.Errorf("genPageMarkdown: Error Fetching html %s:\n", err)
		}
		storefile(filename)
		return renderMarkdown(markdown, 120, theme), nil

	}

	log.Printf("Stat successful. Path %s exists\n", filename)
	log.Printf("Using cached path \"%s\"\n", filename)

	data, err := os.ReadFile(filename)

	if err != nil {
		log.Printf("genPageMarkdown: Error readind file \"%s\" due to %s", filename, err)
		if markdown, err := getHTML(item.Link, item.Title); err == nil {
		return renderMarkdown(markdown, 120, theme), nil
		}
	} else {
	 markdown = string(data)
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
