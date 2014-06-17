package api

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
)

const (
	// Support a single value for PageToken.Limit to make life easy
	tokenLimit = 100
)

type PageToken struct {
	Limit uint16
	Page  uint16
}

func DefaultPageToken() PageToken {
	return PageToken{Limit: tokenLimit, Page: 1}
}

func (tok PageToken) Next() PageToken {
	return PageToken{Limit: tok.Limit, Page: tok.Page + 1}
}

func (tok PageToken) Encode() string {
	buf := bytes.Buffer{}
	binary.Write(&buf, binary.LittleEndian, tok)
	return base64.URLEncoding.EncodeToString(buf.Bytes())
}

func decodePageToken(value string) (*PageToken, error) {
	dec, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	db := bytes.NewBuffer(dec)

	var tok PageToken
	binary.Read(db, binary.LittleEndian, &tok)
	return &tok, nil
}

func findNextPageToken(u *url.URL) (*PageToken, error) {
	values := u.Query()["nextPageToken"]

	if len(values) > 1 {
		return nil, errors.New("too many values for page token")
	}

	if len(values) == 0 {
		return nil, nil
	}

	val := values[0]
	tok, err := decodePageToken(val)
	if err != nil {
		return nil, err
	}

	err = validatePageToken(tok)
	if err != nil {
		return nil, err
	}

	return tok, nil
}

func validatePageToken(tok *PageToken) error {
	if tok.Limit != tokenLimit {
		return fmt.Errorf("token limit must be %d", tokenLimit)
	}

	if tok.Page == 0 {
		return errors.New("token page must be greater than zero")
	}

	return nil
}
