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
	ID            uint64    `db:"id" json:"id"`
	BlockID       uint64    `db:"block_id" json:"block_id"`
	Height        uint64    `db:"height" json:"height"`
	Identifier    string    `db:"identifier" json:"identifier"`
	Version       uint      `db:"version" json:"version"`
	Typ           string    `db:"typ" json:"typ"`
	SenderAddr    string    `db:"sender_addr" json:"sender_addr"`
	RecipientAddr string    `db:"recipient_addr" json:"recipient_addr"`
	Data          Bytea     `db:"data" json:"data"`
	Sign          Bytea     `db:"sign" json:"sign"`
	Fee           uint64    `db:"fee" json:"fee"`
	Hash          string    `db:"hash" json:"hash"`
	InsertedAt    time.Time `db:"inserted_at" json:"inserted_at"`
}

type Bytea struct {
	Bytes  []byte
	Status int
}
