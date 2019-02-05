package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	// Create flags for our optional variables
	port := flag.Int("port", 3000, "the port to start the CYOA web application on")
	filename := flag.String("file", "gopher.json", "the JSON file with the CYOA story")
	flag.Parse()
	fmt.Printf("Using the story in %s.\n", *filename)

	// Open the JSON file and parse the story in it.
	f, err := os.Open(*filename)
	if err != nil {
		panic(err)
	}
	story, err := JsonStory(f)
	if err != nil {
		panic(err)
	}

	// Create our custom CYOA story handler
	tpl := template.Must(template.New("").Parse(storyTmpl))
	h := NewHandler(story,
		WithTemplate(tpl),
		WithPathFunc(pathFn),
	)
	// Create a ServeMux to route our requests
	mux := http.NewServeMux()
	// This story handler is using a custom function and template
	// Because we use /story/ (trailing slash) all web requests
	// whose path has the /story/ prefix will be routed here.
	mux.Handle("/story/", h)
	// This story handler is using the default functions and templates
	// Because we use / (base path) all incoming requests not
	// mapped elsewhere will be sent here.
	mux.Handle("/", NewHandler(story))
	// Start the server using our ServeMux
	fmt.Printf("Starting the server on port: %d\n", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), mux))
}

// Updated chapter parsing function. Technically you don't
// *have* to get the story from the path (it could be a
// header or anything else) but I'm not going to rename this
// since "path" is what we used in the videos.
func pathFn(r *http.Request) string {
	path := strings.TrimSpace(r.URL.Path)
	if path == "/story" || path == "/story/" {
		path = "/story/intro"
	}
	return path[len("/story/"):]
}

// Slightly altered template to show how this feature works
var storyTmpl = `
<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <title>Choose Your Own Adventure</title>
  </head>
  <body>
    <section class="page">
      <h1>{{.Title}}</h1>
      {{range .Paragraphs}}
        <p>{{.}}</p>
      {{end}}
      <ul>
      {{range .Options}}
        <li><a href="/story/{{.Chapter}}">{{.Text}}</a></li>
      {{end}}
      </ul>
    </section>
    <style>
      body {
        font-family: helvetica, arial;
      }
      h1 {
        text-align:center;
        position:relative;
      }
      .page {
        width: 80%;
        max-width: 500px;
        margin: auto;
        margin-top: 40px;
        margin-bottom: 40px;
        padding: 80px;
        background: #FCF6FC;
        border: 1px solid #eee;
        box-shadow: 0 10px 6px -6px #797;
      }
      ul {
        border-top: 1px dotted #ccc;
        padding: 10px 0 0 0;
        -webkit-padding-start: 0;
      }
      li {
        padding-top: 10px;
      }
      a,
      a:visited {
        text-decoration: underline;
        color: #555;
      }
      a:active,
      a:hover {
        color: #222;
      }
      p {
        text-indent: 1em;
      }
    </style>
  </body>
</html>`

func init() {
	tpl = template.Must(template.New("").Parse(defaultHandlerTmpl))
}

var tpl *template.Template

var defaultHandlerTmpl = `
<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <title>Choose Your Own Adventure</title>
  </head>
  <body>
    <section class="page">
      <h1>{{.Title}}</h1>
      {{range .Paragraphs}}
        <p>{{.}}</p>
      {{end}}
      {{if .Options}}
        <ul>
        {{range .Options}}
          <li><a href="/{{.Chapter}}">{{.Text}}</a></li>
        {{end}}
        </ul>
      {{else}}
        <h3>The End</h3>
      {{end}}
    </section>
    <style>
      body {
        font-family: helvetica, arial;
      }
      h1 {
        text-align:center;
        position:relative;
      }
      .page {
        width: 80%;
        max-width: 500px;
        margin: auto;
        margin-top: 40px;
        margin-bottom: 40px;
        padding: 80px;
        background: #FFFCF6;
        border: 1px solid #eee;
        box-shadow: 0 10px 6px -6px #777;
      }
      ul {
        border-top: 1px dotted #ccc;
        padding: 10px 0 0 0;
        -webkit-padding-start: 0;
      }
      li {
        padding-top: 10px;
      }
      a,
      a:visited {
        text-decoration: none;
        color: #6295b5;
      }
      a:active,
      a:hover {
        color: #7792a2;
      }
      p {
        text-indent: 1em;
      }
    </style>
  </body>
</html>`

// HandlerOptions are used with the NewHandler function to configure the http.Handler returned.
type HandlerOption func(h *handler)

// WithTemplate is an option to provide a custom template to
// be used when rendering stories.
func WithTemplate(t *template.Template) HandlerOption {
	return func(h *handler) {
		h.t = t
	}
}

// WithPathFunc is an option to provide a custom function
// for processing the story chapter from the incoming request.
// This should probably be named something like "ChapterParser"
// but leaving it as is since this what I used in the video.
func WithPathFunc(fn func(r *http.Request) string) HandlerOption {
	return func(h *handler) {
		h.pathFn = fn
	}
}

// NewHandler will construct an http.Handler that will render
// the story provided.
// The default handler will use the full path (minus the / prefix)
// as the chapter name, defaulting to "intro" if the path is
// empty. The default template creates option links that follow
// this pattern.
func NewHandler(s Story, opts ...HandlerOption) http.Handler {
	h := handler{s, tpl, defaultPathFn}
	for _, opt := range opts {
		opt(&h)
	}
	return h
}

type handler struct {
	s      Story
	t      *template.Template
	pathFn func(r *http.Request) string
}

func defaultPathFn(r *http.Request) string {
	path := strings.TrimSpace(r.URL.Path)
	if path == "" || path == "/" {
		path = "/intro"
	}
	return path[1:]
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := h.pathFn(r)

	if chapter, ok := h.s[path]; ok {
		err := h.t.Execute(w, chapter)
		if err != nil {
			log.Printf("%v", err)
			http.Error(w, "Something went wrong...", http.StatusInternalServerError)
		}
		return
	}
	http.Error(w, "Chapter not found.", http.StatusNotFound)
}

// JsonStory will decode a story using the incoming reader
// and the encoding/json package. It is assumed that the
// provided reader has the story stored in JSON.
func JsonStory(r io.Reader) (Story, error) {
	d := json.NewDecoder(r)
	var story Story
	if err := d.Decode(&story); err != nil {
		return nil, err
	}
	return story, nil
}

// Story represents a Choose Your Own Adventure story.
// Each key is the name of a story chapter (aka "arc"), and
// each value is a Chapter.
type Story map[string]Chapter

// Chapter represents a CYOA story chapter (or "arc"). Each
// chapter includes its title, the paragraphs it is composed
// of, and options available for the reader to take at the
// end of the chapter. If the options are empty it is
// assumed that you have reached the end of that particular
// story path.
type Chapter struct {
	Title      string   `json:"title"`
	Paragraphs []string `json:"story"`
	Options    []Option `json:"options"`
}

// Option represents a choice offered at the end of a story
// chapter. Text is the visible text end users will see,
// while the Chapter field will be the key to a chapter
// stored in the Story object this chapter was found in.
type Option struct {
	Text    string `json:"text"`
	Chapter string `json:"arc"`
}
