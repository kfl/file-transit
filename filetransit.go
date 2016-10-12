package filetransit

import (
	"io"
	"fmt"
	"net/http"
	"net/url"
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/net/context"

	"google.golang.org/appengine"
	"google.golang.org/appengine/file"
	"google.golang.org/appengine/log"

	"cloud.google.com/go/storage"

)

func serveError(ctx context.Context, w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, "Internal Server Error")
	log.Errorf(ctx, "%v", err)
}

const rootHTML = `<!DOCTYPE html>
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
    <form method="POST" action="/upload" enctype="multipart/form-data">
      Course: <input type="text" name="course" /> <br/>
      Upload File: <input type="file" name="file" /> <br />
      <input type="submit" name="submit" value="Let's Rock and Roll" />
    </form>
  </body>
</html>`

func handleRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, rootHTML)
}

func uniqueFilename(stats string, base string) string {
	randBytes := make([]byte, 32)
    rand.Read(randBytes)
	pre := url.QueryEscape(stats)
	if pre == "" {
		pre = "default"
	}
    return pre+"/"+base64.RawURLEncoding.EncodeToString(randBytes)+"/"+url.QueryEscape(base)
}

var bucket string

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}

	ctx := appengine.NewContext(r)

	f, fh, err := r.FormFile("file")
	if err != nil {
		msg := fmt.Sprintf("Could not get file: %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	defer f.Close()

	if bucket == "" {
        var err error
        if bucket, err = file.DefaultBucketName(ctx); err != nil {
			log.Errorf(ctx, "failed to get default GCS bucket name: %v", err)
			msg := fmt.Sprintf("failed to get default GCS bucket name: %v", err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
        }
	}
	
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		msg := fmt.Sprintf("failed to create client: %v", err)
		http.Error(w, msg, http.StatusInternalServerError)
		log.Errorf(ctx, msg) 
		return
	}
	defer storageClient.Close()

	filename := uniqueFilename(r.FormValue("course"), fh.Filename)
	
	obj := storageClient.Bucket(bucket).Object(filename) 
	sw := obj.NewWriter(ctx)
	if _, err := io.Copy(sw, f); err != nil {
		msg := fmt.Sprintf("Could not write file: %v", err)
		http.Error(w, msg, http.StatusInternalServerError)
		log.Errorf(ctx, msg) 
		return
	}

	if err := sw.Close(); err != nil {
		msg := fmt.Sprintf("Could not put file: %v", err)
		http.Error(w, msg, http.StatusInternalServerError)
		log.Errorf(ctx, msg) 
		return
	}

	obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader)

	u, _ := url.Parse("/" + bucket + "/" + filename)
	dst := fmt.Sprintf("https://storage.googleapis.com%s", u)
	
	http.Redirect(w, r, dst, http.StatusFound)
}


func init() {
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/upload", handleUpload)
}
