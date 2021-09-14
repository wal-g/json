package json

import (
	"encoding/json"
	"io"
)

func Marshal(obj interface{}, buf io.Writer) error {
	marshalled, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = buf.Write(marshalled)
	return err
}
