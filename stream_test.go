// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"strings"
	"testing"

	"github.com/EinKrebs/json/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRawMessage(t *testing.T) {
	var data struct {
		X  float64
		Id RawMessage
		Y  float32
	}
	const raw = `["\u0056",null]`
	const msg = `{"X":0.1,"Id":["\u0056",null],"Y":0.2}`
	require.NoError(t, Unmarshal(strings.NewReader(msg), &data))
	require.Equal(t, raw, string(data.Id))

	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(&data, buf))
	assert.Equal(t, msg, buf.String())
}

func TestNullRawMessage(t *testing.T) {
	var data struct {
		X     float64
		Id    RawMessage
		IdPtr *RawMessage
		Y     float32
	}
	const msg = `{"X":0.1,"Id":null,"IdPtr":null,"Y":0.2}`
	require.NoError(t, Unmarshal(strings.NewReader(msg), &data))
	require.Equal(t, "null", string(data.Id))
	require.Equal(t, (*RawMessage)(nil), data.IdPtr)
	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(&data, buf))
	assert.Equal(t, msg, buf.String())
}
