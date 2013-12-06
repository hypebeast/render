// Package render is a middleware for Martini that provides easy JSON serialization and HTML template rendering.
//
//  package main
//
//  import (
//    "github.com/codegangsta/martini"
//    "github.com/codegangsta/martini-contrib/render"
//  )
//
//  func main() {
//    m := martini.Classic()
//    m.Use(render.Renderer("templates"))
//
//    m.Get("/html", func(r render.Render) {
//      r.HTML(200, "mytemplate", nil)
//    })
//
//    m.Get("/json", func(r render.Render) {
//      r.JSON(200, "hello world")
//    })
//
//    m.Run()
//  }
package render

import (
	"bytes"
	"encoding/json"
	"github.com/codegangsta/martini"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

const (
	ContentType = "Content-Type"
	ContentJSON = "application/json"
	ContentHTML = "text/html"

	defaultDirectory = "templates"
)

// Render is a service that can be injected into a Martini handler. Render provides functions for easily writing JSON and
// HTML templates out to a http Response.
type Render interface {
	// JSON writes the given status and JSON serialized version of the given value to the http.ResponseWriter.
	JSON(status int, v interface{})
	// HTML renders a html template specified by the name and writes the result and given status to the http.ResponseWriter.
	HTML(status int, name string, v interface{})
	// Error is a convenience function that writes an http status to the http.ResponseWriter.
	Error(status int)
}

type Options struct {
	Directory string
}

// Renderer is a Middleware that maps a render.Render service into the Martini handler chain. Renderer will compile templates
// globbed in the given dir. Templates must have the .tmpl extension to be compiled.
//
// If MARTINI_ENV is set to "" or "development" then templates will be recompiled on every request. For more performance, set the
// MARTINI_ENV environment variable to "production"
func Renderer(options ...Options) martini.Handler {
	opt := prepareOptions(options)
	t := compile(opt)
	return func(res http.ResponseWriter, c martini.Context) {
		// recompile for easy development
		if martini.Env == martini.Dev {
			t = compile(opt)
		}
		tc, _ := t.Clone()
		c.MapTo(&renderer{res, tc}, (*Render)(nil))
	}
}

func prepareOptions(options []Options) Options {
	var opt Options
	if len(options) > 0 {
		opt = options[0]
	} else {
		opt = Options{}
	}

	// Defaults
	if opt.Directory == "" {
		opt.Directory = defaultDirectory
	}

	return opt
}

func compile(options Options) *template.Template {
	dir := options.Directory
	t := template.New(dir)

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		r, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		ext := filepath.Ext(r)
		if ext == ".tmpl" {

			buf, err := ioutil.ReadFile(path)
			if err != nil {
				panic(err)
			}

			name := (r[0 : len(r)-len(ext)])
			tmpl := t.New(filepath.ToSlash(name))

			fm := template.FuncMap{
				"yield": func() string {
					return "nope"
				},
			}

			// Bomb out if parse fails. We don't want any silent server starts.
			template.Must(tmpl.Funcs(fm).Parse(string(buf)))
		}

		return nil
	})

	return t
}

type renderer struct {
	http.ResponseWriter
	t *template.Template
}

func (r *renderer) JSON(status int, v interface{}) {
	result, err := json.Marshal(v)
	if err != nil {
		http.Error(r, err.Error(), 500)
		return
	}

	// json rendered fine, write out the result
	r.Header().Set(ContentType, ContentJSON)
	r.WriteHeader(status)
	r.Write(result)
}

func (r *renderer) HTML(status int, name string, binding interface{}) {
	fm := template.FuncMap{
		"yield": func() string {
			var buf bytes.Buffer
			if err := r.t.ExecuteTemplate(&buf, name, binding); err != nil {
				return "nope"
			}
			return buf.String()
		},
	}
	r.t.Funcs(fm)
	var buf bytes.Buffer
	if err := r.t.ExecuteTemplate(&buf, "layout", binding); err != nil {
		http.Error(r, err.Error(), 500)
		return
	}

	// template rendered fine, write out the result
	r.Header().Set(ContentType, ContentHTML)
	r.WriteHeader(status)
	r.Write(buf.Bytes())
}

func (r *renderer) Error(status int) {
	r.WriteHeader(status)
}
