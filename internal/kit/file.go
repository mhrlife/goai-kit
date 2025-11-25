package kit

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

func FileImage(mime string, fileContent []byte) File {
	base64Content := base64.StdEncoding.EncodeToString(fileContent)
	return File{
		DataURI: fmt.Sprintf("data:%s;base64,%s", mime, base64Content),
		Name:    "",
	}
}
