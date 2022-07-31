package pengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func (e *Engine) send(ctx context.Context, body string) (answer, error) {
	var v answer
	r := strings.NewReader(body + "\n.")

	if e.debug {
		log.Printf("pengine(%s) → send0: %s", e.id, body)
	}

	href := fmt.Sprintf("%s/send?format=json&id=%s", e.client.URL, url.QueryEscape(e.id))
	req, err := http.NewRequestWithContext(ctx, "POST", href, r)
	if err != nil {
		return v, err
	}
	req.Header.Set("Content-Type", "application/x-prolog; charset=utf-8")

	resp, err := e.client.client().Do(req)
	if err != nil {
		return v, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return v, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	if e.debug {
		log.Printf("pengine(%s) ← receive json...", e.id)
		dump, _ := httputil.DumpResponse(resp, true)
		log.Println(string(dump))
	}

	err = json.NewDecoder(resp.Body).Decode(&v)
	return v, err
}

func (e *Engine) get(ctx context.Context, action string, format string) (answer, error) {
	var v answer

	params := url.Values{}
	params.Set("id", e.id)
	params.Set("format", format)

	req, err := http.NewRequestWithContext(ctx, "GET", e.client.URL+"/"+action+"?"+params.Encode(), nil)
	if err != nil {
		return v, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := e.client.client().Do(req)
	if err != nil {
		return v, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return v, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	if e.debug {
		log.Printf("pengine(%s) ← got json...", e.id)
		dump, _ := httputil.DumpResponse(resp, true)
		log.Println(string(dump))
	}

	err = json.NewDecoder(resp.Body).Decode(&v)
	return v, err
}

func (e *Engine) post(ctx context.Context, action string, body any) (answer, error) {
	var v answer
	var r io.Reader
	if body != nil {
		bs, err := json.Marshal(body)
		if err != nil {
			return v, err
		}
		r = bytes.NewReader(bs)
	}

	if e.debug {
		log.Printf("pengine(%s) → post JSON: %s", e.id, body)
	}

	var param string
	if e.id != "" {
		param = "?id=" + url.QueryEscape(e.id)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.client.URL+"/"+action+param, r)
	if err != nil {
		return v, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := e.client.client().Do(req)
	if err != nil {
		return v, err
	}
	defer resp.Body.Close()

	// if e.debug {
	// 	rrr, _ := httputil.DumpResponse(resp, true)
	// 	fmt.Println("GOT: ", string(rrr))
	// }

	if resp.StatusCode != http.StatusOK {
		return v, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	if e.debug {
		dump, _ := httputil.DumpResponse(resp, true)
		log.Println(string(dump))
	}

	err = json.NewDecoder(resp.Body).Decode(&v)
	return v, err
}

func (e *Engine) sendProlog(ctx context.Context, body string) (string, error) {
	r := strings.NewReader(body + "\n.")

	if e.debug {
		log.Printf("pengine(%s) → send prolog: %s", e.id, body)
	}

	href := fmt.Sprintf("%s/send?format=prolog&id=%s", e.client.URL, url.QueryEscape(e.id))
	req, err := http.NewRequestWithContext(ctx, "POST", href, r)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-prolog; charset=utf-8")

	resp, err := e.client.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return "", err
	}

	if e.debug {
		log.Printf("pengine(%s) ← got prolog: %s", e.id, buf.String())
	}

	return buf.String(), nil
}

func (e *Engine) postProlog(action string, body any) (string, error) {
	bs, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	r := bytes.NewReader(bs)

	if e.debug {
		log.Printf("pengine(%s) → post prolog: %s", e.id, string(bs))
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s?format=prolog", e.client.URL, action), r)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := e.client.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return "", err
	}

	if e.debug {
		log.Printf("pengine(%s) ← got prolog: %s", e.id, buf.String())
	}

	return buf.String(), nil
}
