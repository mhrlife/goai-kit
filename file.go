package goaikit

import (
	"encoding/base64"
	"fmt"
)

type File struct {
	DataURI string
	Name    string
}

func FilePDF(name string, fileContent []byte) File {
	base64Content := base64.StdEncoding.EncodeToString(fileContent)

	return File{
		DataURI: fmt.Sprintf("data:application/pdf;base64,%s", base64Content),
		Name:    name,
	}
}

func FilePNG(name string, fileContent []byte) File {
	base64Content := base64.StdEncoding.EncodeToString(fileContent)

	return File{
		DataURI: fmt.Sprintf("data:image/png;base64,%s", base64Content),
		Name:    name,
	}
}
