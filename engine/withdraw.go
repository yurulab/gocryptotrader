package engine

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes"
	withdrawDataStore "github.com/yurulab/gocryptotrader/database/repository/withdraw"
	"github.com/yurulab/gocryptotrader/gctrpc"
	"github.com/yurulab/gocryptotrader/log"
	"github.com/yurulab/gocryptotrader/portfolio/withdraw"
)

const (
	// ErrWithdrawRequestNotFound message to display when no record is found
	ErrWithdrawRequestNotFound = "%v not found"
	// ErrRequestCannotbeNil message to display when request is nil
	ErrRequestCannotbeNil = "request cannot be nil"
	// StatusError const for for "error" string
	StatusError = "error"
)

// SubmitWithdrawal preforms validation and submits a new withdraw request to exchange
func SubmitWithdrawal(exchName string, req *withdraw.Request) (*withdraw.Response, error) {
	if req == nil {
		return nil, errors.New(ErrRequestCannotbeNil)
	}

	var err error
	var ret *withdraw.ExchangeResponse
	if req.Exchange == "" {
		req.Exchange = exchName
	}

	err = withdraw.Validate(req)
	if err != nil {
		return nil, err
	}

	exch := GetExchangeByName(exchName)
	if exch == nil {
		return nil, ErrExchangeNotFound
	}

	resp := &withdraw.Response{
		Exchange: &withdraw.ExchangeResponse{
			Name: exchName,
		},
		RequestDetails: req,
	}

	if Bot.Settings.EnableDryRun {
		log.Warnln(log.Global, "Dry run enabled, no withdrawal request will be submitted or have an event created")
		resp.ID = withdraw.DryRunID
		resp.Exchange.Status = "dryrun"
		resp.Exchange.ID = withdraw.DryRunID.String()
	} else {
		if req.Type == withdraw.Fiat {
			ret, err = exch.WithdrawFiatFunds(req)
			if err != nil {
				resp.Exchange.ID = StatusError
				resp.Exchange.Status = err.Error()
			} else {
				resp.Exchange.Status = ret.Status
				resp.Exchange.ID = ret.ID
			}
		} else if req.Type == withdraw.Crypto {
			ret, err = exch.WithdrawCryptocurrencyFunds(req)
			if err != nil {
				resp.Exchange.ID = StatusError
				resp.Exchange.Status = err.Error()
			} else {
				resp.Exchange.Status = ret.Status
				resp.Exchange.ID = ret.ID
			}
		}
		withdrawDataStore.Event(resp)
	}
	if err == nil {
		withdraw.Cache.Add(resp.ID, resp)
	}
	return resp, nil
}

// WithdrawalEventByID returns a withdrawal request by ID
func WithdrawalEventByID(id string) (*withdraw.Response, error) {
	v := withdraw.Cache.Get(id)
	if v != nil {
		return v.(*withdraw.Response), nil
	}

	l, err := withdrawDataStore.GetEventByUUID(id)
	if err != nil {
		return nil, fmt.Errorf(ErrWithdrawRequestNotFound, id)
	}
	withdraw.Cache.Add(id, l)
	return l, nil
}

// WithdrawalEventByExchange returns a withdrawal request by ID
func WithdrawalEventByExchange(exchange string, limit int) ([]*withdraw.Response, error) {
	return withdrawDataStore.GetEventsByExchange(exchange, limit)
}

// WithdrawEventByDate returns a withdrawal request by ID
func WithdrawEventByDate(exchange string, start, end time.Time, limit int) ([]*withdraw.Response, error) {
	return withdrawDataStore.GetEventsByDate(exchange, start, end, limit)
}

// WithdrawalEventByExchangeID returns a withdrawal request by Exchange ID
func WithdrawalEventByExchangeID(exchange, id string) (*withdraw.Response, error) {
	return withdrawDataStore.GetEventByExchangeID(exchange, id)
}

func parseMultipleEvents(ret []*withdraw.Response) *gctrpc.WithdrawalEventsByExchangeResponse {
	v := &gctrpc.WithdrawalEventsByExchangeResponse{}
	for x := range ret {
		tempEvent := &gctrpc.WithdrawalEventResponse{
			Id: ret[x].ID.String(),
			Exchange: &gctrpc.WithdrawlExchangeEvent{
				Name:   ret[x].Exchange.Name,
				Id:     ret[x].Exchange.Name,
				Status: ret[x].Exchange.Status,
			},
			Request: &gctrpc.WithdrawalRequestEvent{
				Currency:    ret[x].RequestDetails.Currency.String(),
				Description: ret[x].RequestDetails.Description,
				Amount:      ret[x].RequestDetails.Amount,
				Type:        int32(ret[x].RequestDetails.Type),
			},
		}

		createdAtPtype, err := ptypes.TimestampProto(ret[x].CreatedAt)
		if err != nil {
			log.Errorf(log.Global, "failed to convert time: %v", err)
		}
		tempEvent.CreatedAt = createdAtPtype

		updatedAtPtype, err := ptypes.TimestampProto(ret[x].UpdatedAt)
		if err != nil {
			log.Errorf(log.Global, "failed to convert time: %v", err)
		}
		tempEvent.UpdatedAt = updatedAtPtype

		if ret[x].RequestDetails.Type == withdraw.Crypto {
			tempEvent.Request.Crypto = new(gctrpc.CryptoWithdrawalEvent)
			tempEvent.Request.Crypto = &gctrpc.CryptoWithdrawalEvent{
				Address:    ret[x].RequestDetails.Crypto.Address,
				AddressTag: ret[x].RequestDetails.Crypto.AddressTag,
				Fee:        ret[x].RequestDetails.Crypto.FeeAmount,
			}
		} else if ret[x].RequestDetails.Type == withdraw.Fiat {
			if ret[x].RequestDetails.Fiat != nil {
				tempEvent.Request.Fiat = new(gctrpc.FiatWithdrawalEvent)
				tempEvent.Request.Fiat = &gctrpc.FiatWithdrawalEvent{
					BankName:      ret[x].RequestDetails.Fiat.Bank.BankName,
					AccountName:   ret[x].RequestDetails.Fiat.Bank.AccountName,
					AccountNumber: ret[x].RequestDetails.Fiat.Bank.AccountNumber,
					Bsb:           ret[x].RequestDetails.Fiat.Bank.BSBNumber,
					Swift:         ret[x].RequestDetails.Fiat.Bank.SWIFTCode,
					Iban:          ret[x].RequestDetails.Fiat.Bank.IBAN,
				}
			}
		}
		v.Event = append(v.Event, tempEvent)
	}
	return v
}

func parseSingleEvents(ret *withdraw.Response) *gctrpc.WithdrawalEventsByExchangeResponse {
	tempEvent := &gctrpc.WithdrawalEventResponse{
		Id: ret.ID.String(),
		Exchange: &gctrpc.WithdrawlExchangeEvent{
			Name:   ret.Exchange.Name,
			Id:     ret.Exchange.Name,
			Status: ret.Exchange.Status,
		},
		Request: &gctrpc.WithdrawalRequestEvent{
			Currency:    ret.RequestDetails.Currency.String(),
			Description: ret.RequestDetails.Description,
			Amount:      ret.RequestDetails.Amount,
			Type:        int32(ret.RequestDetails.Type),
		},
	}
	createdAtPtype, err := ptypes.TimestampProto(ret.CreatedAt)
	if err != nil {
		log.Errorf(log.Global, "failed to convert time: %v", err)
	}
	tempEvent.CreatedAt = createdAtPtype

	updatedAtPtype, err := ptypes.TimestampProto(ret.UpdatedAt)
	if err != nil {
		log.Errorf(log.Global, "failed to convert time: %v", err)
	}
	tempEvent.UpdatedAt = updatedAtPtype

	if ret.RequestDetails.Type == withdraw.Crypto {
		tempEvent.Request.Crypto = new(gctrpc.CryptoWithdrawalEvent)
		tempEvent.Request.Crypto = &gctrpc.CryptoWithdrawalEvent{
			Address:    ret.RequestDetails.Crypto.Address,
			AddressTag: ret.RequestDetails.Crypto.AddressTag,
			Fee:        ret.RequestDetails.Crypto.FeeAmount,
		}
	} else if ret.RequestDetails.Type == withdraw.Fiat {
		if ret.RequestDetails.Fiat != nil {
			tempEvent.Request.Fiat = new(gctrpc.FiatWithdrawalEvent)
			tempEvent.Request.Fiat = &gctrpc.FiatWithdrawalEvent{
				BankName:      ret.RequestDetails.Fiat.Bank.BankName,
				AccountName:   ret.RequestDetails.Fiat.Bank.AccountName,
				AccountNumber: ret.RequestDetails.Fiat.Bank.AccountNumber,
				Bsb:           ret.RequestDetails.Fiat.Bank.BSBNumber,
				Swift:         ret.RequestDetails.Fiat.Bank.SWIFTCode,
				Iban:          ret.RequestDetails.Fiat.Bank.IBAN,
			}
		}
	}

	return &gctrpc.WithdrawalEventsByExchangeResponse{
		Event: []*gctrpc.WithdrawalEventResponse{tempEvent},
	}
}
