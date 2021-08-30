package withdraw

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/gofrs/uuid"
	"github.com/yurulab/gocryptotrader/currency"
	"github.com/yurulab/gocryptotrader/database"
	modelPSQL "github.com/yurulab/gocryptotrader/database/models/postgres"
	modelSQLite "github.com/yurulab/gocryptotrader/database/models/sqlite3"
	"github.com/yurulab/gocryptotrader/database/repository"
	"github.com/yurulab/gocryptotrader/log"
	"github.com/yurulab/gocryptotrader/portfolio/banking"
	"github.com/yurulab/gocryptotrader/portfolio/withdraw"
	"github.com/thrasher-corp/sqlboiler/boil"
	"github.com/thrasher-corp/sqlboiler/queries/qm"
)

var (
	// ErrNoResults is the error returned if no results are found
	ErrNoResults = errors.New("no results found")
)

// Event stores Withdrawal Response details in database
func Event(res *withdraw.Response) {
	if database.DB.SQL == nil {
		return
	}

	ctx := context.Background()
	ctx = boil.SkipTimestamps(ctx)

	tx, err := database.DB.SQL.BeginTx(ctx, nil)
	if err != nil {
		log.Errorf(log.DatabaseMgr, "Event transaction being failed: %v", err)
		return
	}

	if repository.GetSQLDialect() == database.DBSQLite3 {
		err = addSQLiteEvent(ctx, tx, res)
	} else {
		err = addPSQLEvent(ctx, tx, res)
	}
	if err != nil {
		log.Errorf(log.DatabaseMgr, "Event insert failed: %v", err)
		err = tx.Rollback()
		if err != nil {
			log.Errorf(log.DatabaseMgr, "Event Transaction rollback failed: %v", err)
		}
		return
	}

	err = tx.Commit()
	if err != nil {
		log.Errorf(log.DatabaseMgr, "Event Transaction commit failed: %v", err)
		err = tx.Rollback()
		if err != nil {
			log.Errorf(log.DatabaseMgr, "Event Transaction rollback failed: %v", err)
		}
		return
	}
}

func addPSQLEvent(ctx context.Context, tx *sql.Tx, res *withdraw.Response) (err error) {
	var tempEvent = modelPSQL.WithdrawalHistory{
		Exchange:     res.Exchange.Name,
		ExchangeID:   res.Exchange.ID,
		Status:       res.Exchange.Status,
		Currency:     res.RequestDetails.Currency.String(),
		Amount:       res.RequestDetails.Amount,
		WithdrawType: int(res.RequestDetails.Type),
	}

	if res.RequestDetails.Description != "" {
		tempEvent.Description.SetValid(res.RequestDetails.Description)
	}

	err = tempEvent.Insert(ctx, tx, boil.Infer())
	if err != nil {
		log.Errorf(log.DatabaseMgr, "Event Insert failed: %v", err)
		err = tx.Rollback()
		if err != nil {
			log.Errorf(log.DatabaseMgr, "Rollback failed: %v", err)
		}
		return
	}

	if res.RequestDetails.Type == withdraw.Fiat {
		fiatEvent := &modelPSQL.WithdrawalFiat{
			BankName:          res.RequestDetails.Fiat.Bank.BankName,
			BankAddress:       res.RequestDetails.Fiat.Bank.BankAddress,
			BankAccountName:   res.RequestDetails.Fiat.Bank.AccountName,
			BankAccountNumber: res.RequestDetails.Fiat.Bank.AccountNumber,
			BSB:               res.RequestDetails.Fiat.Bank.BSBNumber,
			SwiftCode:         res.RequestDetails.Fiat.Bank.SWIFTCode,
			Iban:              res.RequestDetails.Fiat.Bank.IBAN,
		}
		err = tempEvent.SetWithdrawalFiatWithdrawalFiats(ctx, tx, true, fiatEvent)
		if err != nil {
			log.Errorf(log.DatabaseMgr, "Event Insert failed: %v", err)
			err = tx.Rollback()
			if err != nil {
				log.Errorf(log.DatabaseMgr, "Rollback failed: %v", err)
			}
			return
		}
	}

	if res.RequestDetails.Type == withdraw.Crypto {
		cryptoEvent := &modelPSQL.WithdrawalCrypto{
			Address: res.RequestDetails.Crypto.Address,
			Fee:     res.RequestDetails.Crypto.FeeAmount,
		}
		if res.RequestDetails.Crypto.AddressTag != "" {
			cryptoEvent.AddressTag.SetValid(res.RequestDetails.Crypto.AddressTag)
		}
		err = tempEvent.AddWithdrawalCryptoWithdrawalCryptos(ctx, tx, true, cryptoEvent)
		if err != nil {
			log.Errorf(log.DatabaseMgr, "Event Insert failed: %v", err)
			err = tx.Rollback()
			if err != nil {
				log.Errorf(log.DatabaseMgr, "Rollback failed: %v", err)
			}
			return
		}
	}

	realID, _ := uuid.FromString(tempEvent.ID)
	res.ID = realID

	return nil
}

func addSQLiteEvent(ctx context.Context, tx *sql.Tx, res *withdraw.Response) (err error) {
	newUUID, errUUID := uuid.NewV4()
	if errUUID != nil {
		log.Errorf(log.DatabaseMgr, "Failed to generate UUID: %v", errUUID)
		err = tx.Rollback()
		if err != nil {
			log.Errorf(log.DatabaseMgr, "Rollback failed: %v", err)
		}
		return
	}

	var tempEvent = modelSQLite.WithdrawalHistory{
		ID:           newUUID.String(),
		Exchange:     res.Exchange.Name,
		ExchangeID:   res.Exchange.ID,
		Status:       res.Exchange.Status,
		Currency:     res.RequestDetails.Currency.String(),
		Amount:       res.RequestDetails.Amount,
		WithdrawType: int64(res.RequestDetails.Type),
	}

	if res.RequestDetails.Description != "" {
		tempEvent.Description.SetValid(res.RequestDetails.Description)
	}

	err = tempEvent.Insert(ctx, tx, boil.Infer())
	if err != nil {
		log.Errorf(log.DatabaseMgr, "Event Insert failed: %v", err)
		err = tx.Rollback()
		if err != nil {
			log.Errorf(log.DatabaseMgr, "Rollback failed: %v", err)
		}
		return
	}

	if res.RequestDetails.Type == withdraw.Fiat {
		fiatEvent := &modelSQLite.WithdrawalFiat{
			BankName:          res.RequestDetails.Fiat.Bank.BankName,
			BankAddress:       res.RequestDetails.Fiat.Bank.BankAddress,
			BankAccountName:   res.RequestDetails.Fiat.Bank.AccountName,
			BankAccountNumber: res.RequestDetails.Fiat.Bank.AccountNumber,
			BSB:               res.RequestDetails.Fiat.Bank.BSBNumber,
			SwiftCode:         res.RequestDetails.Fiat.Bank.SWIFTCode,
			Iban:              res.RequestDetails.Fiat.Bank.IBAN,
		}

		err = tempEvent.AddWithdrawalFiats(ctx, tx, true, fiatEvent)
		if err != nil {
			log.Errorf(log.DatabaseMgr, "Event Insert failed: %v", err)
			err = tx.Rollback()
			if err != nil {
				log.Errorf(log.DatabaseMgr, "Rollback failed: %v", err)
			}
			return
		}
	}

	if res.RequestDetails.Type == withdraw.Crypto {
		cryptoEvent := &modelSQLite.WithdrawalCrypto{
			Address: res.RequestDetails.Crypto.Address,
			Fee:     res.RequestDetails.Crypto.FeeAmount,
		}

		if res.RequestDetails.Crypto.AddressTag != "" {
			cryptoEvent.AddressTag.SetValid(res.RequestDetails.Crypto.AddressTag)
		}

		err = tempEvent.AddWithdrawalCryptos(ctx, tx, true, cryptoEvent)
		if err != nil {
			log.Errorf(log.DatabaseMgr, "Event Insert failed: %v", err)
			err = tx.Rollback()
			if err != nil {
				log.Errorf(log.DatabaseMgr, "Rollback failed: %v", err)
			}
			return
		}
	}

	res.ID = newUUID
	return nil
}

// GetEventByUUID return requested withdraw information by ID
func GetEventByUUID(id string) (*withdraw.Response, error) {
	resp, err := getByColumns(generateWhereQuery([]string{"id"}, []string{id}, 1))
	if err != nil {
		return nil, err
	}
	return resp[0], nil
}

// GetEventsByExchange returns all withdrawal requests by exchange
func GetEventsByExchange(exchange string, limit int) ([]*withdraw.Response, error) {
	return getByColumns(generateWhereQuery([]string{"exchange"}, []string{exchange}, limit))
}

// GetEventByExchangeID return requested withdraw information by Exchange ID
func GetEventByExchangeID(exchange, id string) (*withdraw.Response, error) {
	resp, err := getByColumns(generateWhereQuery([]string{"exchange", "exchange_id"}, []string{exchange, id}, 1))
	if err != nil {
		return nil, err
	}
	return resp[0], err
}

// GetEventsByDate returns requested withdraw information by date range
func GetEventsByDate(exchange string, start, end time.Time, limit int) ([]*withdraw.Response, error) {
	betweenQuery := generateWhereBetweenQuery("created_at", start, end, limit)
	if exchange == "" {
		return getByColumns(betweenQuery)
	}
	return getByColumns(append(generateWhereQuery([]string{"exchange"}, []string{exchange}, 0), betweenQuery...))
}

func generateWhereQuery(columns, id []string, limit int) []qm.QueryMod {
	var queries []qm.QueryMod
	if limit > 0 {
		queries = append(queries, qm.Limit(limit))
	}
	for x := range columns {
		queries = append(queries, qm.Where(columns[x]+"= ?", id[x]))
	}
	return queries
}

func generateWhereBetweenQuery(column string, start, end interface{}, limit int) []qm.QueryMod {
	return []qm.QueryMod{
		qm.Limit(limit),
		qm.Where(column+" BETWEEN ? AND ?", start, end),
	}
}

func getByColumns(q []qm.QueryMod) ([]*withdraw.Response, error) {
	if database.DB.SQL == nil {
		return nil, database.ErrDatabaseSupportDisabled
	}

	var resp []*withdraw.Response
	var ctx = context.Background()
	if repository.GetSQLDialect() == database.DBSQLite3 {
		v, err := modelSQLite.WithdrawalHistories(q...).All(ctx, database.DB.SQL)
		if err != nil {
			return nil, err
		}
		for x := range v {
			var tempResp = &withdraw.Response{}
			newUUID, _ := uuid.FromString(v[x].ID)
			tempResp.ID = newUUID
			tempResp.Exchange = new(withdraw.ExchangeResponse)
			tempResp.Exchange.ID = v[x].ExchangeID
			tempResp.Exchange.Name = v[x].Exchange
			tempResp.Exchange.Status = v[x].Status
			tempResp.RequestDetails = new(withdraw.Request)
			tempResp.RequestDetails = &withdraw.Request{
				Currency:    currency.NewCode(v[x].Currency),
				Description: v[x].Description.String,
				Amount:      v[x].Amount,
				Type:        withdraw.RequestType(v[x].WithdrawType),
			}

			createdAtTime, err := time.Parse(time.RFC3339, v[x].CreatedAt)
			if err != nil {
				log.Errorf(log.DatabaseMgr, "record: %v has an incorrect time format ( %v ) - defaulting to empty time: %v", tempResp.ID, v[x].CreatedAt, err)
				tempResp.CreatedAt = time.Time{}
			} else {
				tempResp.CreatedAt = createdAtTime
			}

			updatedAtTime, err := time.Parse(time.RFC3339, v[x].UpdatedAt)
			if err != nil {
				log.Errorf(log.DatabaseMgr, "record: %v has an incorrect time format ( %v ) - defaulting to empty time: %v", tempResp.ID, v[x].UpdatedAt, err)
				tempResp.UpdatedAt = time.Time{}
			} else {
				tempResp.UpdatedAt = updatedAtTime
			}

			if withdraw.RequestType(v[x].WithdrawType) == withdraw.Crypto {
				x, err := v[x].WithdrawalCryptos().One(ctx, database.DB.SQL)
				if err != nil {
					return nil, err
				}
				tempResp.RequestDetails.Crypto = new(withdraw.CryptoRequest)
				tempResp.RequestDetails.Crypto.Address = x.Address
				tempResp.RequestDetails.Crypto.AddressTag = x.AddressTag.String
				tempResp.RequestDetails.Crypto.FeeAmount = x.Fee
			} else {
				x, err := v[x].WithdrawalFiats().One(ctx, database.DB.SQL)
				if err != nil {
					return nil, err
				}
				tempResp.RequestDetails.Fiat = new(withdraw.FiatRequest)
				tempResp.RequestDetails.Fiat.Bank = new(banking.Account)
				tempResp.RequestDetails.Fiat.Bank.AccountName = x.BankAccountName
				tempResp.RequestDetails.Fiat.Bank.AccountNumber = x.BankAccountNumber
				tempResp.RequestDetails.Fiat.Bank.IBAN = x.Iban
				tempResp.RequestDetails.Fiat.Bank.SWIFTCode = x.SwiftCode
				tempResp.RequestDetails.Fiat.Bank.BSBNumber = x.BSB
			}
			resp = append(resp, tempResp)
		}
	} else {
		v, err := modelPSQL.WithdrawalHistories(q...).All(ctx, database.DB.SQL)
		if err != nil {
			return nil, err
		}

		for x := range v {
			var tempResp = &withdraw.Response{}
			newUUID, _ := uuid.FromString(v[x].ID)
			tempResp.ID = newUUID
			tempResp.Exchange = new(withdraw.ExchangeResponse)
			tempResp.Exchange.ID = v[x].ExchangeID
			tempResp.Exchange.Name = v[x].Exchange
			tempResp.Exchange.Status = v[x].Status
			tempResp.RequestDetails = new(withdraw.Request)
			tempResp.RequestDetails = &withdraw.Request{
				Currency:    currency.NewCode(v[x].Currency),
				Description: v[x].Description.String,
				Amount:      v[x].Amount,
				Type:        withdraw.RequestType(v[x].WithdrawType),
			}
			tempResp.CreatedAt = v[x].CreatedAt
			tempResp.UpdatedAt = v[x].UpdatedAt

			if withdraw.RequestType(v[x].WithdrawType) == withdraw.Crypto {
				tempResp.RequestDetails.Crypto = new(withdraw.CryptoRequest)
				x, err := v[x].WithdrawalCryptoWithdrawalCryptos().One(ctx, database.DB.SQL)
				if err != nil {
					return nil, err
				}
				tempResp.RequestDetails.Crypto.Address = x.Address
				tempResp.RequestDetails.Crypto.AddressTag = x.AddressTag.String
				tempResp.RequestDetails.Crypto.FeeAmount = x.Fee
			} else if withdraw.RequestType(v[x].WithdrawType) == withdraw.Fiat {
				tempResp.RequestDetails.Fiat = new(withdraw.FiatRequest)
				x, err := v[x].WithdrawalFiatWithdrawalFiats().One(ctx, database.DB.SQL)
				if err != nil {
					return nil, err
				}
				tempResp.RequestDetails.Fiat.Bank = new(banking.Account)
				tempResp.RequestDetails.Fiat.Bank.AccountName = x.BankAccountName
				tempResp.RequestDetails.Fiat.Bank.AccountNumber = x.BankAccountNumber
				tempResp.RequestDetails.Fiat.Bank.IBAN = x.Iban
				tempResp.RequestDetails.Fiat.Bank.SWIFTCode = x.SwiftCode
				tempResp.RequestDetails.Fiat.Bank.BSBNumber = x.BSB
			}
			resp = append(resp, tempResp)
		}
	}
	if len(resp) == 0 {
		return nil, ErrNoResults
	}
	return resp, nil
}
