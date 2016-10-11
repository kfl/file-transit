package filetransit

import (
        "html/template"
        "io"
        "net/http"

        "golang.org/x/net/context"

        "google.golang.org/appengine"
        "google.golang.org/appengine/blobstore"
        "google.golang.org/appengine/log"
)

func serveError(ctx context.Context, w http.ResponseWriter, err error) {
        w.WriteHeader(http.StatusInternalServerError)
        w.Header().Set("Content-Type", "text/plain")
        io.WriteString(w, "Internal Server Error")
        log.Errorf(ctx, "%v", err)
}

var rootTemplate = template.Must(template.New("root").Parse(rootTemplateHTML))

const rootTemplateHTML = `<!DOCTYPE html>
<html>
  <head>
    <title>File Transit Storage</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style type="text/css">
     body{ 
       margin:40px auto;
       max-width:650px;
       line-height:1.6;
       font-size:18px;
       font: sans-serif;
       color:#444;
       padding:0 10px
     }
     h1,h2,h3 {
       line-height:1.2
     }
    </style>
  </head>
  <body>
    <h1>For all your file transit needs</h1>
    <form method="POST" action="{{.}}" enctype="multipart/form-data">
      Upload File: <input type="file" name="file" /> <br />
      <input type="submit" name="submit" value="Let's Rock and Roll" />
    </form>
  </body>
</html>`

func handleRoot(w http.ResponseWriter, r *http.Request) {
        ctx := appengine.NewContext(r)
        uploadURL, err := blobstore.UploadURL(ctx, "/upload", nil)
        if err != nil {
                serveError(ctx, w, err)
                return
        }
        w.Header().Set("Content-Type", "text/html")
        err = rootTemplate.Execute(w, uploadURL)
        if err != nil {
                log.Errorf(ctx, "%v", err)
        }
}

func handleServe(w http.ResponseWriter, r *http.Request) {
        blobstore.Send(w, appengine.BlobKey(r.FormValue("blobKey")))
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
        ctx := appengine.NewContext(r)
        blobs, _, err := blobstore.ParseUpload(r)
        if err != nil {
                serveError(ctx, w, err)
                return
        }
        file := blobs["file"]
        if len(file) == 0 {
                log.Errorf(ctx, "no file uploaded")
                http.Redirect(w, r, "/", http.StatusFound)
                return
        }
        http.Redirect(w, r, "/serve/?blobKey="+string(file[0].BlobKey), http.StatusFound)
}

func init() {
        http.HandleFunc("/", handleRoot)
        http.HandleFunc("/serve/", handleServe)
        http.HandleFunc("/upload", handleUpload)
}
