package filetransit

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	s "strings"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/api/iterator"
	"google.golang.org/appengine"
	"google.golang.org/appengine/file"
	"google.golang.org/appengine/log"

	"cloud.google.com/go/storage"
)

func internalError(ctx context.Context, w http.ResponseWriter, msg string, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, msg)
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
	pre := stats
	if pre == "" || pre == "trash" {
		pre = "default"
	}
	return "live/" + pre + "/" + base64.RawURLEncoding.EncodeToString(randBytes) + "/" + base
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
		internalError(ctx, w, "No file uploaded", err)
		return
	}
	defer f.Close()

	if bucket == "" {
		var err error
		if bucket, err = file.DefaultBucketName(ctx); err != nil {
			internalError(ctx, w, "Failed to get default GCS bucket name", err)
			return
		}
	}

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		internalError(ctx, w, "Failed to create GCS client", err)
		return
	}
	defer storageClient.Close()

	filename := uniqueFilename(r.FormValue("course"), fh.Filename)

	obj := storageClient.Bucket(bucket).Object(filename)
	sw := obj.NewWriter(ctx)
	if _, err := io.Copy(sw, f); err != nil {
		internalError(ctx, w, "Could not write file to GCS", err)
		return
	}

	if err := sw.Close(); err != nil {
		internalError(ctx, w, "Could not *put* file to GCS", err)
		return
	}

	obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader)

	dst := fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucket, filename)

	http.Redirect(w, r, dst, http.StatusFound)
}

func handleCleanup(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	if bucket == "" {
		var err error
		if bucket, err = file.DefaultBucketName(ctx); err != nil {
			internalError(ctx, w, "Failed to get default GCS bucket name", err)
			return
		}
	}

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		internalError(ctx, w, "Failed to create GCS client", err)
		return
	}
	defer storageClient.Close()

	buck := storageClient.Bucket(bucket)
	query := &storage.Query{Prefix: "live/"}
	objs := buck.Objects(ctx, query)
	for {
		objAttrs, err := objs.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			internalError(ctx, w, "Failure while iterating through the bucket", err)
			return
		}

		if time.Since(objAttrs.Created).Minutes() > 10 {
			src := buck.Object(objAttrs.Name)
			dst := buck.Object(s.Replace(objAttrs.Name, "live", "trash", 1))
			_, err = dst.CopierFrom(src).Run(ctx)
			if err != nil {
				log.Errorf(ctx, "Error while trying to delete %s: %v", objAttrs.Name, err)
			}
			src.Delete(ctx)
		}

	}
}

func init() {
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/cleanup-task", handleCleanup)
	http.HandleFunc("/upload", handleUpload)
}
