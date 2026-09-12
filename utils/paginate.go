package utils

import (
	"fmt"
	"math"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PaginationLink struct {
	URL    interface{} `json:"url"`
	Label  string      `json:"label"`
	Active bool        `json:"active"`
}

type LaravelCollection struct {
	Data  interface{} `json:"data"`
	Links struct {
		First interface{} `json:"first"`
		Last  interface{} `json:"last"`
		Prev  interface{} `json:"prev"`
		Next  interface{} `json:"next"`
	} `json:"links"`
	Meta struct {
		CurrentPage int              `json:"current_page"`
		From        int              `json:"from"`
		LastPage    int              `json:"last_page"`
		Links       []PaginationLink `json:"links"`
		Path        string           `json:"path"`
		PerPage     int              `json:"per_page"`
		To          int              `json:"to"`
		Total       int              `json:"total"`
	} `json:"meta"`
}

func BuildLaravelPagination(c *gin.Context, data []map[string]interface{}, total int, page int, perPage int) LaravelCollection {
	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage < 1 {
		lastPage = 1
	}
	if page < 1 {
		page = 1
	}

	offset := (page - 1) * perPage

	// Helper build URL giữ nguyên các query params khác (search, filter...)
	buildURL := func(p int) string {
		if p < 1 || p > lastPage {
			return ""
		}
		u, _ := url.Parse(fmt.Sprintf("https://%s%s", c.Request.Host, c.Request.URL.Path))
		q := c.Request.URL.Query()
		q.Set("page", strconv.Itoa(p))
		u.RawQuery = q.Encode()
		return u.String()
	}

	res := LaravelCollection{Data: data}

	// Links
	res.Links.First = buildURL(1)
	res.Links.Last = buildURL(lastPage)
	res.Links.Prev = buildURL(page - 1)
	res.Links.Next = buildURL(page + 1)

	// Meta
	res.Meta.CurrentPage = page
	res.Meta.PerPage = perPage
	res.Meta.Total = total
	res.Meta.LastPage = lastPage
	res.Meta.Path = fmt.Sprintf("https://%s%s", c.Request.Host, c.Request.URL.Path)
	res.Meta.From = offset + 1
	res.Meta.To = offset + len(data)

	// Meta Links (Dùng cho giao diện số trang 1, 2, 3...)
	res.Meta.Links = append(res.Meta.Links, PaginationLink{
		URL:    res.Links.Prev,
		Label:  "&laquo; Previous",
		Active: false,
	})

	for i := 1; i <= lastPage; i++ {
		res.Meta.Links = append(res.Meta.Links, PaginationLink{
			URL:    buildURL(i),
			Label:  strconv.Itoa(i),
			Active: i == page,
		})
	}

	res.Meta.Links = append(res.Meta.Links, PaginationLink{
		URL:    res.Links.Next,
		Label:  "Next &raquo;",
		Active: false,
	})

	return res
}
