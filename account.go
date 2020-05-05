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
	"bytes"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// StoreAccount stores an account.  It will fail if it cannot store the data.
// Note this will overwrite an existing account with the same ID.  It will not, however, allow multiple accounts with the same
// name to co-exist in the same wallet.
func (s *Store) StoreAccount(walletID uuid.UUID, accountID uuid.UUID, data []byte) error {
	// Ensure the wallet exists
	_, err := s.RetrieveWalletByID(walletID)
	if err != nil {
		return errors.New("unknown wallet")
	}

	// See if an account with this name already exists
	existingAccount, err := s.RetrieveAccount(walletID, accountID)
	if err == nil {
		// It does; they need to have the same ID for us to overwrite it
		info := &struct {
			ID string `json:"uuid"`
		}{}
		err := json.Unmarshal(existingAccount, info)
		if err != nil {
			return err
		}
		if info.ID != accountID.String() {
			return errors.New("account already exists")
		}
	}

	data, err = s.encryptIfRequired(data)
	if err != nil {
		return err
	}

	path := s.accountPath(walletID, accountID)
	uploader := s3manager.NewUploader(s.session)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return errors.Wrap(err, "failed to store key")
	}
	return nil
}

// RetrieveAccount retrieves account-level data.  It will fail if it cannot retrieve the data.
func (s *Store) RetrieveAccount(walletID uuid.UUID, accountID uuid.UUID) ([]byte, error) {
	path := s.accountPath(walletID, accountID)
	buf := aws.NewWriteAtBuffer([]byte{})
	downloader := s3manager.NewDownloader(s.session)
	if _, err := downloader.Download(buf,
		&s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(path),
		}); err != nil {
		return nil, err
	}
	data, err := s.decryptIfRequired(buf.Bytes())
	if err != nil {
		return nil, err
	}
	return data, nil
}

// RetrieveAccounts retrieves all account-level data for a wallet.
func (s *Store) RetrieveAccounts(walletID uuid.UUID) <-chan []byte {
	path := s.walletPath(walletID)
	ch := make(chan []byte, 1024)
	go func() {
		conn := s3.New(s.session)
		resp, err := conn.ListObjectsV2(&s3.ListObjectsV2Input{
			Bucket: aws.String(s.bucket),
			Prefix: aws.String(path + "/"),
		})
		if err == nil {
			for _, item := range resp.Contents {
				if strings.HasSuffix(*item.Key, "/") {
					// Directory
					continue
				}
				if strings.HasSuffix(*item.Key, walletID.String()) {
					// Wallet
					continue
				}
				buf := aws.NewWriteAtBuffer([]byte{})
				downloader := s3manager.NewDownloader(s.session)
				_, err := downloader.Download(buf,
					&s3.GetObjectInput{
						Bucket: aws.String(s.bucket),
						Key:    aws.String(*item.Key),
					})
				if err != nil {
					continue
				}
				data, err := s.decryptIfRequired(buf.Bytes())
				if err != nil {
					continue
				}
				ch <- data
			}
		}
		close(ch)
	}()
	return ch
}
