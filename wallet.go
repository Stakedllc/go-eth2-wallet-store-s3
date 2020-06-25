// Copyright 2019, 2020 Weald Technology Trading
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vault

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// StoreWallet stores wallet-level data.  It will fail if it cannot store the data.
// Note that this will overwrite any existing data; it is up to higher-level functions to check for the presence of a wallet with
// the wallet name and handle clashes accordingly.
func (s *Store) StoreWallet(id uuid.UUID, name string, data []byte) error {
	path := s.walletHeaderPath(id.String())
	s.Authorize()

	client := s.client
	var err error

	_, err = client.Logical().WriteBytes(path, data)

	if err != nil {
		return errors.Wrap(err, "failed to store wallet")
	}
	return nil
}

// RetrieveWallet retrieves wallet-level data.  It will fail if it cannot retrieve the data.
func (s *Store) RetrieveWallet(walletName string) ([]byte, error) {
	for data := range s.RetrieveWallets() {
		info := &struct {
			Name string `json:"name"`
		}{}
		err := json.Unmarshal(data, info)
		if err == nil && info.Name == walletName {
			return data, nil
		}
	}
	return nil, errors.New("wallet not found")
}

// RetrieveWalletByID retrieves wallet-level data.  It will fail if it cannot retrieve the data.
func (s *Store) RetrieveWalletByID(walletID uuid.UUID) ([]byte, error) {
	for data := range s.RetrieveWallets() {
		info := &struct {
			ID uuid.UUID `json:"uuid"`
		}{}
		err := json.Unmarshal(data, info)
		if err == nil && info.ID == walletID {
			return data, nil
		}
	}
	return nil, errors.New("wallet not found")
}

// RetrieveWallets retrieves wallet-level data for all wallets.
func (s *Store) RetrieveWallets() <-chan []byte {
	ch := make(chan []byte, 1024)
	s.Authorize()

	client := s.client

	go func() {
		secret, err := client.Logical().List(s.walletsPath())

		if err != nil || secret == nil {
			close(ch)
			return
		}

		wallets, typeError := secret.Data["keys"].([]interface{})

		if !typeError {
			close(ch)
			return
		}

		for _, wallet := range wallets {
			walletName := wallet.(string)
			nameLength := len(walletName) - 1
			// Quietly skip these errors
			// TODO: Handle errors better through the channel
			secret, err := client.Logical().Read(s.walletHeaderPath(walletName[:nameLength]))

			if err != nil {
				continue
			}

			byteData, err := json.Marshal(secret.Data)

			if err != nil {
				continue
			}

			ch <- byteData
		}

		close(ch)
	}()
	return ch
}
