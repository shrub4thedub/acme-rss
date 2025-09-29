package main

import (
    "bufio"
    "fmt"
    "log"
    "net/http"
    "net/url"
    "os"
    "path/filepath"
    "strconv"
    "strings"

    "9fans.net/go/acme"
    "github.com/go-shiori/go-readability"
    "github.com/mmcdole/gofeed"
)

type feedSource struct {
    Name string
    URL  string
}

func main() {
    feeds, err := readFeedsFile()
    if err != nil {
        log.Fatalf("Failed to read ~/.rss: %v", err)
    }
    if len(feeds) == 0 {
        log.Fatal("No feeds found in ~/.rss")
    }

    win, err := acme.New()
    if err != nil {
        log.Fatal(err)
    }
    win.Name("rss")

    h := &rssHandler{
        win:   win,
        feeds: feeds,
    }

    if err := win.OpenEvent(); err != nil {
        log.Fatal(err)
    }

    // Show the first feed by default
    h.current = feeds[0]
    h.reload()

    win.Ctl("clean")
    win.EventLoop(h)
}

type rssHandler struct {
    win     *acme.Win
    items   []*gofeed.Item
    feeds   []feedSource
    current feedSource
}

func (h *rssHandler) Execute(cmd string) bool {
    for _, f := range h.feeds {
        if cmd == f.Name {
            h.current = f
            h.reload()
            return true
        }
    }
    if cmd == "Reload" {
        h.reload()
        return true
    }
    if id, err := strconv.Atoi(cmd); err == nil {
        h.openItem(id - 1)
        return true
    }
    return false
}

func (h *rssHandler) Look(arg string) bool {
    if id, err := strconv.Atoi(arg); err == nil {
        h.openItem(id - 1)
        return true
    }
    return false
}

func (h *rssHandler) reload() {
    fp := gofeed.NewParser()
    feed, err := fp.ParseURL(h.current.URL)
    if err != nil {
        h.win.Clear()
        h.win.Fprintf("body", "Failed to load feed %s: %v\n", h.current.URL, err)
        h.win.Ctl("clean")
        return
    }

    h.items = feed.Items
    h.win.Clear()

    // Build tag parts, highlighting current
    var tagParts []string
    for _, f := range h.feeds {
        if f.Name == h.current.Name {
            tagParts = append(tagParts, "["+f.Name+"]")
        } else {
            tagParts = append(tagParts, f.Name)
        }
    }
    tagParts = append(tagParts, "Reload")


    h.win.Ctl("cleartag")

    //write new tag parts
    h.win.Write("tag", []byte(strings.Join(tagParts, " ")))

    // write content
    h.win.Fprintf("body", "Feed: %s (%s)\n\n", feed.Title, h.current.URL)
    for i, item := range feed.Items {
        date := ""
        if item.PublishedParsed != nil {
            date = item.PublishedParsed.Format("2006-01-02")
        }
        h.win.Fprintf("body", "%d/ %s (%s)\n", i+1, item.Title, date)
    }

    h.win.Ctl("clean")
}

func (h *rssHandler) openItem(index int) {
    if index < 0 || index >= len(h.items) {
        return
    }
    item := h.items[index]
    link := item.Link

    w, err := acme.New()
    if err != nil {
        log.Println("New window error:", err)
        return
    }
    w.Name(fmt.Sprintf("rss/item/%d", index+1))
    w.Fprintf("body", "%s\n\n%s\n\nLink: %s\n\n", item.Title, item.Description, link)

    text, err := fetchReadableText(link)
    if err != nil || text == "" {
        w.Fprintf("body", "[Could not fetch full article — showing RSS summary instead]\n\n%s\n", item.Description)
    } else {
        w.Fprintf("body", "%s\n", text)
    }

    w.Ctl("clean")

    go func() {
        if err := w.OpenEvent(); err != nil {
            log.Println("OpenEvent failed:", err)
            return
        }
        w.EventLoop(&messageHandler{win: w})
    }()
}

func fetchReadableText(link string) (string, error) {
    resp, err := http.Get(link)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    baseURL, err := url.Parse(link)
    if err != nil {
        return "", err
    }

    article, err := readability.FromReader(resp.Body, baseURL)
    if err != nil {
        return "", err
    }
    return article.TextContent, nil
}

func readFeedsFile() ([]feedSource, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, err
    }

    path := filepath.Join(home, ".rss")
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var feeds []feedSource
    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        parts := strings.Fields(line)
        if len(parts) == 1 {
            feeds = append(feeds, feedSource{
                Name: fmt.Sprintf("Feed%d", len(feeds)+1),
                URL:  parts[0],
            })
        } else {
            feeds = append(feeds, feedSource{
                Name: parts[0],
                URL:  parts[1],
            })
        }
    }
    return feeds, scanner.Err()
}

type messageHandler struct {
    win *acme.Win
}

func (m *messageHandler) Execute(cmd string) bool { return false }
func (m *messageHandler) Look(arg string) bool   { return false }
