// Command files uploads a file, lists files, retrieves it and deletes it. With
// no argument it uploads a generated PNG.
//
//	go run ./examples/files [path/to/image.png]
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/files"
)

func main() {
	payload := ex.SamplePNG()
	filename := "sample.png"
	contentType := "image/png"
	if len(os.Args) > 1 {
		path := os.Args[1]
		data, err := os.ReadFile(path)
		ex.Must(err)
		payload = data
		filename = filepath.Base(path)
		contentType = ""
	}

	ctx := context.Background()
	client := files.New(ex.NewClient())
	created, err := client.Create(ctx, &files.CreateRequest{
		Purpose:     files.PurposeUserData,
		File:        payload,
		Filename:    filename,
		ContentType: contentType,
	})
	ex.Must(err)
	fmt.Printf("uploaded %s (%d bytes)\n", created.ID, created.Bytes)

	list, err := client.List(ctx, &files.ListOptions{Limit: new(5)})
	ex.Must(err)
	for _, file := range list.Data {
		fmt.Printf("  %s %s %d bytes\n", file.ID, file.Filename, file.Bytes)
	}

	retrieved, err := client.Retrieve(ctx, created.ID)
	ex.Must(err)
	fmt.Printf("retrieved %s (%s)\n", retrieved.ID, retrieved.Filename)

	deleted, err := client.Delete(ctx, created.ID)
	ex.Must(err)
	fmt.Printf("deleted=%v %s\n", deleted.Deleted, deleted.ID)
}
