// package tcabcireadgoclient
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

package tcabcireadgoclient

import "time"

type Type string

func (t Type) IsValid() bool {
	valid := false

	for i := 0; i < len(TypesSlice); i++ {
		if TypesSlice[i] == t {
			valid = true
			break
		}
	}

	return valid
}

const (
	TypeMaster                Type = "initial_storage"
	TypeAddress               Type = "interim_storage"
	TypeAddresses             Type = "interim_storages"
	TypeSubMaster             Type = "initial_sub_storage"
	TypeSubAddresses          Type = "interim_sub_storages"
	TypeAccount               Type = "initial_account"
	TypeAccountInitialTXS     Type = "initial_account_txs"
	TypeMessage               Type = "message"
	TypeMessageSent           Type = "inherit_message"
	TypeMessageThreadDelete   Type = "inherit_message_recv"
	TypeTransfer              Type = "transfer"
	TypeTransferCancel        Type = "transfer_Cancel"
	TypeTransferSent          Type = "transfer_sent"
	TypeTransferReceiveDelete Type = "transfer_receive_delete"
	TypeTransferInfo          Type = "transfer_info"
	TypeStorage               Type = "storage"
	TypeStorageDelete         Type = "storage_delete"
	TypeBackup                Type = "backup"
	TypeContact               Type = "interim_message"
	TypeFileVirtual           Type = "fs_virt"
	TypeFileFs                Type = "fs_real"
	TypeRfileVirtual          Type = "fs_rvirt"
	TypeRfileFs               Type = "fs_rreal"
	TypeDfileVirtual          Type = "fs_dvirt"
	TypeDfileFs               Type = "fs_dreal"
	TypePfileVirtual          Type = "fs_pvirt"
	TypeRequest               Type = "request"
	TypeRequestIn             Type = "request_in"
	TypeRequestUpload         Type = "request_upload"
	TypeRequestCancel         Type = "request_Cancel"
	TypeDataRoom              Type = "data_room"
	TypeDataRoomPolicy        Type = "data_room_policy"
	TypeDataRoomF             Type = "data_roomF"
	TypeDataRoomData          Type = "data_room_data"
	TypeDataRoomDataDelete    Type = "data_room_data_delete"
	TypeDataRoomDataPolicy    Type = "data_room_data_policy"
	TypeMultiStorage          Type = "multi_storage"
	TypeMultiTransfer         Type = "multi_transfer"
	TypeMultiTransferSent     Type = "multi_transfer_sent"
	TypeMultiBackup           Type = "multi_backup"
	TypeMultiDataRoom         Type = "multi_data_room"
	TypePasswdData            Type = "passwd_data"
	TypePasswdRoom            Type = "passwd_room"
	TypePasswdRoomPolicy      Type = "passwd_room_policy"
	TypePasswdRoomF           Type = "passwd_roomF"
	TypePasswdRoomData        Type = "passwd_room_data"
	TypePasswdRoomDataDelete  Type = "passwd_room_data_delete"
	TypePasswdRoomDataPolicy  Type = "passwd_room_data_policy"
	TypePasswdDataV2          Type = "pwdd"
	TypePasswdDataV2Policy    Type = "pwdd_policy"
)

var TypesSlice = []Type{
	TypeMaster,
	TypeAddress,
	TypeAddresses,
	TypeSubMaster,
	TypeSubAddresses,
	TypeAccount,
	TypeAccountInitialTXS,
	TypeMessage,
	TypeMessageSent,
	TypeMessageThreadDelete,
	TypeTransfer,
	TypeTransferCancel,
	TypeTransferSent,
	TypeTransferReceiveDelete,
	TypeTransferInfo,
	TypeStorage,
	TypeStorageDelete,
	TypeBackup,
	TypeContact,
	TypeFileVirtual,
	TypeFileFs,
	TypeRfileVirtual,
	TypeRfileFs,
	TypeDfileVirtual,
	TypeDfileFs,
	TypePfileVirtual,
	TypeRequest,
	TypeRequestIn,
	TypeRequestUpload,
	TypeRequestCancel,
	TypeDataRoom,
	TypeDataRoomPolicy,
	TypeDataRoomF,
	TypeDataRoomData,
	TypeDataRoomDataDelete,
	TypeDataRoomDataPolicy,
	TypeMultiStorage,
	TypeMultiTransfer,
	TypeMultiTransferSent,
	TypeMultiBackup,
	TypeMultiDataRoom,
	TypePasswdData,
	TypePasswdRoom,
	TypePasswdRoomPolicy,
	TypePasswdRoomF,
	TypePasswdRoomData,
	TypePasswdRoomDataDelete,
	TypePasswdRoomDataPolicy,
	TypePasswdDataV2,
	TypePasswdDataV2Policy,
}

// Transaction read node transaction model
type Transaction struct {
	Order         *uint64     `json:"order,omitempty"`
	ID            interface{} `json:"id"`
	Height        uint64      `json:"height"`
	Identifier    string      `json:"identifier"`
	Version       uint        `json:"version"`
	Typ           Type        `json:"typ"`
	SenderAddr    string      `json:"sender_addr"`
	RecipientAddr string      `json:"recipient_addr"`
	Data          Bytea       `json:"data"`
	Sign          Bytea       `json:"sign"`
	Fee           uint64      `json:"fee"`
	Hash          string      `json:"hash"`
	InsertedAt    time.Time   `json:"inserted_at"`
}

// Bytea transaction byte data
type Bytea struct {
	Bytes  []byte
	Status int
}
