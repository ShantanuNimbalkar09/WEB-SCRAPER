package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type seoData struct {
	URL             string
	Title           string
	H1              string
	MetaDescription string
	StatusCode      int
}

type Parser interface {
	getSEOData(resp *http.Response) (seoData, error)
}

type DefaultParser struct {
}

var userAgents = []string{
	"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; Googlebot/2.1; +http://www.google.com/bot.html) Chrome/W.X.Y.Z Safari/537.36",
}

func isSiteMap(urls []string) ([]string, []string) {
	sitemapFiles := []string{}
	pages := []string{}
	for _, page := range urls {
		foundsitemap := strings.Contains(page, "xml")
		if foundsitemap == true {
			fmt.Println("Found SiteMap", page)
			sitemapFiles = append(sitemapFiles, page)
		} else {
			pages = append(pages, page)
		}
	}
	return sitemapFiles, pages
}

func randomUserAgent() string {
	rand.Seed(time.Now().Unix())
	randnum := rand.Int() % len(userAgents)
	return userAgents[randnum]
}

func makeRequest(url string) (*http.Response, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", randomUserAgent())
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil

}

func extractSiteMapURLs(startURL string) []string {
	Worklist := make(chan []string)
	toCrawl := []string{}
	var n int
	n++
	go func() { Worklist <- []string{startURL} }()

	for ; n > 0; n-- {
		list := <-Worklist
		for _, link := range list {
			n++
			go func(link string) {
				response, err := makeRequest(link)
				if err != nil {
					log.Printf("Error retriving URL:%s", link)
				}
				urls, _ := extractUrls(response)
				if err != nil {
					log.Printf("Error extracting document from response,URL:%s", link)
				}

				sitemapFiles, pages := isSiteMap(urls)
				if sitemapFiles != nil {
					Worklist <- sitemapFiles
				}
				for _, page := range pages {
					toCrawl = append(toCrawl, page)
				}
			}(link)
		}
	}

	return toCrawl

}

func scrapeURLs(urls []string, parser Parser, concurrency int) []seoData {
	tokens := make(chan struct{}, concurrency)
	var n int

	Worklist := make(chan []string)
	results := []seoData{}
	go func() {
		Worklist <- urls
	}()
	for ; n > 0; n-- {
		list := <-Worklist
		for _, url := range list {
			if url != "" {
				n++
				go func(url string, token chan struct{}) {
					log.Printf("Requesting Url:%s", url)
					res, err := scrapePage(url, tokens, parser)
					if err != nil {
						log.Printf("Encountered Error,URL:%s", url)
					} else {
						results = append(results, res)
					}

					Worklist <- []string{}
				}(url, tokens)
			}
		}
	}
	return results

}

func extractUrls(resp *http.Response) ([]string, error) {
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return nil, err
	}
	results := []string{}
	sel := doc.Find("loc")
	for i := range sel.Nodes {
		loc := sel.Eq(i)
		result := loc.Text()
		results = append(results, result)
	}

	return results, nil

}

func scrapePage(url string, token chan struct{}, parser Parser) (seoData, error) {
	res, err := crawlPage(url, token)
	if err != nil {
		return seoData{}, err
	}
	data, err := parser.getSEOData(res)

	if err != nil {
		return seoData{}, err
	}
	return data, nil
}

func crawlPage(url string, tokens chan struct{}) (*http.Response, error) {
	tokens <- struct{}{}
	resp, err := makeRequest(url)
	<-tokens

	if err != nil {
		return nil, err
	}
	return resp, nil

}

func (d DefaultParser) getSEOData(resp *http.Response) (seoData, error) {

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return seoData{}, err
	}

	result := seoData{}
	result.URL = resp.Request.URL.String()
	result.StatusCode = resp.StatusCode
	result.Title = doc.Find("title").First().Text()
	result.H1 = doc.Find("h1").First().Text()
	result.MetaDescription, _ = doc.Find("meta[name^=description]").Attr("content")
	return result, nil
}

func ScrapeSitemap(url string, parser Parser, concurrency int) []seoData {

	results := extractSiteMapURLs(url)
	res := scrapeURLs(results, parser, concurrency)
	return res
}

func main() {
	p := DefaultParser{}
	results := ScrapeSitemap("https://www.quicksprout.com/sitemap.xml", p, 10)
	for _, res := range results {
		fmt.Println(res)
	}
}
