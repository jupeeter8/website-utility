package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"
)

var rss Rss

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

func fetchXML() (Rss, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get(fmt.Sprintf("%s/feed", cfg.BaseUrl))
	if err != nil {
		log.Printf("fetchXML: failed to fetch rss feed %s\n", err)
		return Rss{}, err
	}
	defer resp.Body.Close()
	log.Println("Successfully fetched xml")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("fetchXML: failure in reading body of the response %s\n", err)
		return Rss{}, err
	}
	var rss Rss
	if err := xml.Unmarshal(body, &rss); err != nil {
		log.Printf("fetchXML: failure in parsing XML body %s\n", err)
		return Rss{}, err
	}
	log.Println("XML Unmarshal successful")
	return rss, nil
}

func updateRssList(updates chan Rss) bool {
	log.Println("getXML: performing an rss fetch")
	rssFeed, err := fetchXML()
	if err != nil {
		log.Printf("getXML: fetch failed: %s", err)
		return false
	}
	updates <- rssFeed
	return true

}

func sortFeed() {
	sort.Slice(rss.Channel.ItemList, func(i, j int) bool {
		timeI, err1 := time.Parse(time.RFC1123, rss.Channel.ItemList[i].PubDate)
		timeJ, err2 := time.Parse(time.RFC1123, rss.Channel.ItemList[j].PubDate)

		if err1 != nil || err2 != nil {
			return false
		}
		return timeJ.After(timeI)

	})
}

func getXML() {

	var updates = make(chan Rss, 1)
	var ticker = time.Tick(30 * time.Minute)

	for {
		failures := 0
		if failures > 2 {
			panic("Failure to fetch feed, exiting")
		}
		fetchSuccess := updateRssList(updates)
		if fetchSuccess {
			log.Println("Intial fetch complete")
			break
		} else {
			failures++
			log.Println("initial fetch failed trying again after 3 seconds")
			time.Sleep(3 * time.Second)
		}

	}

	for {
		select {
		case rss = <-updates:
			sortFeed()
		case <-ticker:
			updateRssList(updates)
		}
	}

}
