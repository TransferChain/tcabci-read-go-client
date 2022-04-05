// Package tcabci_read_go_client
//
// Copyright 2019 TransferChain A.G
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tcabci_read_go_client

import "time"

type Transaction struct {
	ID            uint64    `json:"id"`
	BlockID       uint64    `json:"block_id"`
	Height        uint64    `json:"height"`
	Identifier    string    `json:"identifier"`
	Version       uint      `json:"version"`
	Typ           string    `json:"typ"`
	SenderAddr    string    `json:"sender_addr"`
	RecipientAddr string    `json:"recipient_addr"`
	Data          Bytea     `json:"data"`
	Sign          Bytea     `json:"sign"`
	Fee           uint64    `json:"fee"`
	Hash          string    `json:"hash"`
	InsertedAt    time.Time `json:"inserted_at"`
}

type Bytea struct {
	Bytes  []byte
	Status int
}
