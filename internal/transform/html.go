// Package transform contains the HTML transformation logic for the DarkHN proxy.
// It rewrites Hacker News HTML to apply a dark theme, strip login-gated features,
// and fix all resource URLs to be absolute.
package transform

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

const (
	MirrorHost     = "news.ycombinator.com"
	MirrorProtocol = "https"
	GithubURL      = "https://github.com/xtrp/darkhn"
)

// HTML transforms an HN HTML page into the dark-themed proxy version.
// mirrorURL is the original upstream URL (used for "light" links and comment forms).
func HTML(r io.Reader, mirrorURL string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return "", fmt.Errorf("parsing HTML: %w", err)
	}

	// Rewrite link[href] to absolute URLs (stylesheets, icons, feeds).
	doc.Find("link[href]").Each(func(_ int, s *goquery.Selection) {
		if href, ok := s.Attr("href"); ok {
			s.SetAttr("href", toAbsolute(href))
		}
	})

	// Rewrite script[src] and img[src] to absolute URLs.
	doc.Find("script[src], img[src]").Each(func(_ int, s *goquery.Selection) {
		if src, ok := s.Attr("src"); ok {
			s.SetAttr("src", toAbsolute(src))
		}
	})

	// Rewrite a[href] with the HN hostname to relative paths, keeping
	// other hrefs (relative or external) untouched.
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok {
			return
		}
		if isHNAbsolute(href) {
			u, err := url.Parse(href)
			if err != nil {
				return
			}
			rel := u.Path
			if u.RawQuery != "" {
				rel += "?" + u.RawQuery
			}
			if rel == "" {
				rel = "/"
			}
			s.SetAttr("href", rel)
		}
	})

	// Append " Dark" to the page title.
	doc.Find("title").Each(func(_ int, s *goquery.Selection) {
		s.SetText(s.Text() + " Dark")
	})

	// Strip vote links; keep the visual arrow but remove the clickable anchor.
	doc.Find(".votelinks").Each(func(_ int, s *goquery.Selection) {
		s.SetHtml(`<center><div class="votearrow"></div></center>`)
	})

	// Remove the "submit" link from the nav bar together with its preceding " | " text.
	doc.Find(`.pagetop a[href^="submit"], .pagetop a[href^="/submit"]`).Each(func(_ int, s *goquery.Selection) {
		n := s.Nodes[0]
		if n.PrevSibling != nil && n.PrevSibling.Type == html.TextNode {
			n.Parent.RemoveChild(n.PrevSibling)
		}
		s.Remove()
	})

	// Replace the login cell with links to the light site and the darkhn repo.
	doc.Find("td:last-child > .pagetop").SetHtml(fmt.Sprintf(
		"\n        <a href=\"%s\">light</a>\n         | \n        <a href=\"%s\">darkhn</a>",
		mirrorURL, GithubURL,
	))

	// Remove "hide" links and their trailing " | " text sibling.
	doc.Find(`a[href^="hide"], a[href^="/hide"]`).Each(func(_ int, s *goquery.Selection) {
		n := s.Nodes[0]
		if n.NextSibling != nil && n.NextSibling.Type == html.TextNode {
			n.Parent.RemoveChild(n.NextSibling)
		}
		s.Remove()
	})

	// Remove "fave" links and their trailing " | " text sibling.
	doc.Find(`a[href^="fave"], a[href^="/fave"]`).Each(func(_ int, s *goquery.Selection) {
		n := s.Nodes[0]
		if n.NextSibling != nil && n.NextSibling.Type == html.TextNode {
			n.Parent.RemoveChild(n.NextSibling)
		}
		s.Remove()
	})

	// Remove the <br> that follows .fatitem to fix spacing.
	doc.Find(".fatitem + br").Remove()

	// Append an empty anchor placeholder to the footer links bar.
	doc.Find(".yclinks").AppendHtml(` | <a></a>`)

	// On item/comment pages: change reply links to open on the default site.
	doc.Find(".reply a").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok {
			return
		}
		s.SetText("reply on default site")
		s.SetAttr("target", "_blank")

		replyURL := toAbsolute(href)
		u, err := url.Parse(replyURL)
		if err != nil {
			return
		}
		if gotoParam := u.Query().Get("goto"); gotoParam != "" {
			q := u.Query()
			q.Set("goto", toAbsolute(gotoParam))
			u.RawQuery = q.Encode()
		}
		s.SetAttr("href", u.String())
	})

	// On item/comment pages: replace comment submission forms with a link to
	// submit on the default site.
	doc.Find("form[action=comment]").Each(func(_ int, s *goquery.Selection) {
		actionName := s.Find(`input[type=submit]`).AttrOr("value", "add comment")
		parent := s.Parent()
		parent.SetHtml(fmt.Sprintf(
			`<a href="%s" target='_blank' class='comment-on-default-site'>%s on default site</a>`,
			mirrorURL, actionName,
		))
		parent.Parent().Prev().SetAttr("style", "height: 20px;")
	})

	// Inject the dark-mode stylesheet at the end of <head>.
	doc.Find("head").AppendHtml(`<link rel="stylesheet" href="inject.css" />`)

	// Render the innerHTML of <html> (same as cheerio's $('html').html()).
	inner, err := doc.Find("html").Html()
	if err != nil {
		return "", fmt.Errorf("rendering HTML: %w", err)
	}
	return inner, nil
}

// toAbsolute converts a URL that lacks a scheme+host into an absolute URL
// pointing to the HN mirror. URLs that already have a scheme or are
// protocol-relative (//) are returned unchanged.
func toAbsolute(u string) string {
	if u == "" || strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "//") {
		return u
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	return MirrorProtocol + "://" + MirrorHost + u
}

// isHNAbsolute reports whether href is an absolute URL targeting the HN mirror.
func isHNAbsolute(href string) bool {
	return strings.HasPrefix(href, "http://"+MirrorHost) ||
		strings.HasPrefix(href, "https://"+MirrorHost)
}
