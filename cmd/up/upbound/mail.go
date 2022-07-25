// Copyright 2022 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package upbound

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/mail"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/upbound/up/internal/install"
)

const (
	mailSecretType = "type=upbound.io/email"
	mailMessageKey = "message"
	mailTmplPath   = "templates/mail.html"

	errGetMessages = "failed to get messages"
	errBadMessage  = "skipping incorrectly formatted message"

	errGetMessage   = "failed to get message"
	errParseMessage = "failed to parse message"
	errReadBody     = "failed to read message body"

	errWriteResponse = "failed to write response"

	failedGetMessages  = "Could not find messages."
	failedGetMessage   = "Could not get message."
	failedParseMessage = "Could not parse message."
)

var reqTimeout = 10 * time.Second

// MailItem is metadata for an email message.
type MailItem struct {
	Recipient string
	Subject   string
}

//go:embed templates/mail.html
var f embed.FS

// AfterApply sets default values in command after assignment and validation.
func (c *mailCmd) AfterApply(insCtx *install.Context) error {
	c.log = logging.NewLogrLogger(zap.New(zap.UseDevMode(c.Verbose)))
	client, err := kubernetes.NewForConfig(insCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client
	c.ns = insCtx.Namespace
	b, err := f.ReadFile(mailTmplPath)
	if err != nil {
		return err
	}
	t, err := template.New("mail").Parse(string(b))
	if err != nil {
		return err
	}
	c.tmpl = t
	return nil
}

// mailCmd runs the Upbound Mail Portal.
type mailCmd struct {
	log     logging.Logger
	kClient kubernetes.Interface
	ns      string
	tmpl    *template.Template

	Port    int  `default:"8085" short:"p" help:"Port used for mail portal."`
	Verbose bool `help:"Run server with verbose logging."`
}

// Run executes the mail command.
func (c *mailCmd) Run(kCtx *kong.Context) error {
	s := &http.Server{
		Handler:           http.HandlerFunc(c.handler),
		Addr:              fmt.Sprintf("127.0.0.1:%d", c.Port),
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Printf("Running Mail Portal on port: %d\n", c.Port)
	go func() {
		if err := s.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			kCtx.FatalIfErrorf(err)
		}
	}()
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	<-done
	c.log.Debug("Shutting down Mail server.")
	to, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.Shutdown(to)
}

func (c *mailCmd) handler(rw http.ResponseWriter, r *http.Request) { //nolint:gocyclo
	ctx, cancel := context.WithTimeout(r.Context(), reqTimeout)
	defer cancel()
	if r.URL.Path == "/" {
		l, err := c.kClient.CoreV1().Secrets(c.ns).List(ctx, v1.ListOptions{
			FieldSelector: mailSecretType,
		})
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			if _, err := rw.Write([]byte(failedGetMessages)); err != nil {
				c.log.WithValues("Error", err).Info(errWriteResponse)
			}
			c.log.WithValues("Error", err).Debug(errGetMessages)
			return
		}
		msgs := make(map[string]MailItem, len(l.Items))
		for _, s := range l.Items {
			msg, err := mail.ReadMessage(bytes.NewBuffer(s.Data[mailMessageKey]))
			if err != nil {
				c.log.WithValues("Message", s.Name, "Error", err).Debug(errBadMessage)
				continue
			}
			msgs[s.Name] = MailItem{
				Recipient: msg.Header.Get("To"),
				Subject:   msg.Header.Get("Subject"),
			}
		}

		if err := c.tmpl.Execute(rw, msgs); err != nil {
			c.log.WithValues("Error", err).Info(errWriteResponse)
		}
		return
	}
	s, err := c.kClient.CoreV1().Secrets(c.ns).Get(ctx, strings.TrimLeft(r.URL.Path, "/"), v1.GetOptions{})
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		if _, err := rw.Write([]byte(failedGetMessage)); err != nil {
			c.log.WithValues("Error", err).Info(errWriteResponse)
		}
		c.log.WithValues("Error", err).Debug(errGetMessage)
		return
	}
	msg, err := mail.ReadMessage(bytes.NewBuffer(s.Data[mailMessageKey]))
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		if _, err := rw.Write([]byte(failedParseMessage)); err != nil {
			c.log.WithValues("Error", err).Info(errWriteResponse)
		}
		c.log.WithValues("Error", err).Debug(errParseMessage)
		return
	}
	b, err := io.ReadAll(msg.Body)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		if _, err := rw.Write([]byte(failedParseMessage)); err != nil {
			c.log.WithValues("Error", err).Info(errWriteResponse)
		}
		c.log.WithValues("Error", err).Debug(errReadBody)
		return
	}
	if _, err := rw.Write(b); err != nil {
		c.log.WithValues("Error", err).Info(errWriteResponse)
	}
}
