package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httputil"

	"github.com/jordan-wright/email"
	"github.com/sirupsen/logrus"
)

type Hook struct {
	Config  *Config
	From    string
	To      []string
	Options string
	Email   *email.Email
}

func (h *Hook) Do() {
	logrus.Debugf("hooking %s -> %s", h.From, h.To)
	h.Config.wg.Add(1)
	defer h.Config.wg.Done()

	c, b := h.body()
	err := h.send(c, b)
	if err != nil {
		// TODO: retry on error
		logrus.Errorf("error on sending hook: %s", err)
	}
}

func (h *Hook) body() (string, *bytes.Buffer) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)

	w.WriteField("from", h.From)
	w.WriteField("to", fmt.Sprintf("%v", h.To))
	w.WriteField("options", h.Options)
	w.WriteField("subject", h.Email.Subject)
	w.WriteField("text", string(h.Email.Text))
	w.WriteField("html", string(h.Email.HTML))

	for _, at := range h.Email.Attachments {
		aw, err := w.CreateFormFile("attachment[]", at.Filename)
		if err != nil {
			logrus.Errorf("error attach file from message: %s", err)
		}
		i, err := io.Copy(aw, bytes.NewReader(at.Content))
		if err != nil {
			logrus.Errorf("can not copy email attachment to http request: ", err)
		}

		logrus.Debugf("bytes copied %d", i)
	}

	w.Close()
	return w.FormDataContentType(), body
}

func (h *Hook) send(contentType string, body io.Reader) error {
	r, _ := http.NewRequest("POST", h.Config.URI, body)
	r.Header.Add("Content-Type", contentType)

	if logrus.GetLevel() >= logrus.DebugLevel {
		reqDump, err := httputil.DumpRequestOut(r, true)
		if err != nil {
			logrus.Errorf("err : %s", err)
		}
		fmt.Printf("REQUEST:\n%s", string(reqDump))
	}

	client := &http.Client{}
	resp, err := client.Do(r)
	if err != nil {
		return err
	}

	if resp.StatusCode > 299 {
		return fmt.Errorf("http status code :%d", resp.StatusCode)
	}

	return nil
}
