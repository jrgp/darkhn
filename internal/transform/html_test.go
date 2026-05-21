package transform

import (
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openTestFile opens a file relative to the repo root (two levels up from this package).
func openTestFile(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("../../" + name)
	require.NoError(t, err, "opening test file %s", name)
	return f
}

func TestHTMLTransform_SourceToReference(t *testing.T) {
	src := openTestFile(t, "source.html")
	defer src.Close()

	out, err := HTML(src, "https://"+MirrorHost+"/")
	require.NoError(t, err)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(out))
	require.NoError(t, err)

	t.Run("title_has_Dark_suffix", func(t *testing.T) {
		title := doc.Find("title").Text()
		assert.True(t, strings.HasSuffix(title, " Dark"), "title %q should end with ' Dark'", title)
	})

	t.Run("inject_css_present", func(t *testing.T) {
		found := doc.Find(`link[href="inject.css"]`).Length()
		assert.Equal(t, 1, found, "inject.css link should be injected once")
	})

	t.Run("link_hrefs_are_absolute", func(t *testing.T) {
		doc.Find("link[href]").Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			if href == "inject.css" {
				return // our own injected stylesheet is intentionally relative
			}
			assert.True(t,
				strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "//"),
				"link href %q should be absolute", href,
			)
		})
	})

	t.Run("script_srcs_are_absolute", func(t *testing.T) {
		doc.Find("script[src]").Each(func(_ int, s *goquery.Selection) {
			src, _ := s.Attr("src")
			assert.True(t,
				strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "//"),
				"script src %q should be absolute", src,
			)
		})
	})

	t.Run("img_srcs_are_absolute", func(t *testing.T) {
		doc.Find("img[src]").Each(func(_ int, s *goquery.Selection) {
			src, _ := s.Attr("src")
			assert.True(t,
				strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "//"),
				"img src %q should be absolute", src,
			)
		})
	})

	t.Run("no_hn_absolute_links_in_anchors", func(t *testing.T) {
		// The only absolute HN anchor we allow is the intentionally injected
		// "light" link that points back to the real site.
		doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
			if s.Text() == "light" {
				return
			}
			href, _ := s.Attr("href")
			assert.False(t, isHNAbsolute(href), "anchor href %q should not be absolute HN URL", href)
		})
	})

	t.Run("vote_links_removed", func(t *testing.T) {
		count := doc.Find(`.votelinks a`).Length()
		assert.Equal(t, 0, count, "all vote anchor links should be removed")
	})

	t.Run("votearrow_div_kept", func(t *testing.T) {
		count := doc.Find(`.votelinks .votearrow`).Length()
		assert.Greater(t, count, 0, "votearrow divs should remain inside .votelinks")
	})

	t.Run("submit_link_removed", func(t *testing.T) {
		count := doc.Find(`.pagetop a[href^="submit"], .pagetop a[href^="/submit"]`).Length()
		assert.Equal(t, 0, count, "submit link should be removed from nav")
	})

	t.Run("light_link_present", func(t *testing.T) {
		found := doc.Find(`a[href="https://` + MirrorHost + `/"]`).Length()
		assert.Equal(t, 1, found, "light link pointing to HN should be present")
	})

	t.Run("darkhn_github_link_present", func(t *testing.T) {
		found := doc.Find(`a[href="` + GithubURL + `"]`).Length()
		assert.Equal(t, 1, found, "darkhn GitHub link should be present")
	})

	t.Run("login_link_removed", func(t *testing.T) {
		count := doc.Find(`a[href^="login"]`).Length()
		assert.Equal(t, 0, count, "login link should be replaced")
	})

	t.Run("hide_links_removed", func(t *testing.T) {
		count := doc.Find(`a[href^="hide"], a[href^="/hide"]`).Length()
		assert.Equal(t, 0, count, "hide links should be removed")
	})

	t.Run("fave_links_removed", func(t *testing.T) {
		count := doc.Find(`a[href^="fave"], a[href^="/fave"]`).Length()
		assert.Equal(t, 0, count, "fave links should be removed")
	})

	t.Run("yclinks_empty_anchor_appended", func(t *testing.T) {
		// The last child of .yclinks should be an <a> with no href (the appended placeholder).
		last := doc.Find(".yclinks a").Last()
		_, hasHref := last.Attr("href")
		assert.False(t, hasHref, "last .yclinks anchor should be the empty placeholder <a></a>")
	})
}

func TestToAbsolute(t *testing.T) {
	cases := []struct{ in, want string }{
		{"news.css?x=1", "https://news.ycombinator.com/news.css?x=1"},
		{"/y18.svg", "https://news.ycombinator.com/y18.svg"},
		{"https://example.com/foo", "https://example.com/foo"},
		{"http://example.com/foo", "http://example.com/foo"},
		{"//hn.algolia.com/", "//hn.algolia.com/"},
		{"", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, toAbsolute(c.in), "toAbsolute(%q)", c.in)
	}
}

func TestIsHNAbsolute(t *testing.T) {
	assert.True(t, isHNAbsolute("https://news.ycombinator.com/news"))
	assert.True(t, isHNAbsolute("http://news.ycombinator.com/"))
	assert.False(t, isHNAbsolute("/news"))
	assert.False(t, isHNAbsolute("https://example.com/news"))
}
