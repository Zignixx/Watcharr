package jikan

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	gocache "github.com/robfig/go-cache"

	"github.com/sbondCo/Watcharr/cache"
)

// In-memory content cache
var MangaStore = gocache.New(time.Hour*24, time.Minute)

const (
	jikanHost = "https://api.jikan.moe/v4"
)

type Jikan struct{}

func NewJikan() *Jikan {
	return &Jikan{}
}

func (j *Jikan) req(ep string, params map[string]string, resp interface{}) error {
	slog.Debug("Jikan->req: Creating a request.", "ep", ep, "params", params)

	base, err := url.Parse(jikanHost)
	if err != nil {
		return errors.New("failed to parse api uri")
	}

	base.Path += ep

	qp := url.Values{}
	for k, v := range params {
		qp.Add(k, v)
	}
	base.RawQuery = qp.Encode()

	slog.Debug("Jikan->req", "url", base.String())

	req, err := http.NewRequest("GET", base.String(), nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return err
	}
	if !(res.StatusCode >= 200 && res.StatusCode <= 299) {
		slog.Error("Jikan non 2xx status code:", "status_code", res.StatusCode)
		return errors.New(string(body))
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return err
	}
	return nil
}

func (j *Jikan) Search(q string) (JikanSearchResponse, error) {
	slog.Debug("Jikan Search:", "query", q)
	var resp JikanSearchResponse
	cacheKey := cache.CreateCacheKey("JikanSearch", q)
	if cache.GetCache(MangaStore, cacheKey, &resp) {
		slog.Debug("Jikan Search: Returning cache.")
		return resp, nil
	}
	err := j.req("/manga", map[string]string{"q": q, "limit": "25"}, &resp)
	if err != nil {
		slog.Error("Jikan Search: request failed!", "error", err)
		return JikanSearchResponse{}, errors.New("request failed")
	}
	MangaStore.Set(cacheKey, resp, time.Hour*24)
	return resp, nil
}

func (j *Jikan) MangaDetails(id int) (JikanManga, error) {
	slog.Debug("Jikan MangaDetails:", "id", id)
	cacheKey := cache.CreateCacheKey("JikanMangaDetails", strconv.Itoa(id))
	var cachedResp JikanMangaDetailsResponse
	if cache.GetCache(MangaStore, cacheKey, &cachedResp) {
		slog.Debug("Jikan MangaDetails: Returning cache.")
		return cachedResp.Data, nil
	}
	var resp JikanMangaDetailsResponse
	err := j.req("/manga/"+strconv.Itoa(id), map[string]string{}, &resp)
	if err != nil {
		slog.Error("Jikan MangaDetails: request failed!", "error", err)
		return JikanManga{}, errors.New("request failed")
	}
	MangaStore.Set(cacheKey, resp, time.Hour*24)
	return resp.Data, nil
}

// Response wrapper for single manga details
type JikanMangaDetailsResponse struct {
	Data JikanManga `json:"data"`
}
