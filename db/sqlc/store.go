package db

import (
	"context"
	"database/sql"
	"fmt"
)

//store provides all functions to execute db query and transaction

type Store struct {
	*Queries
	db *sql.DB
}

//newstore creates a new store
//takes a sql db as input, and returns a store
//inside build a new store object, db is input sql db, and query is calling the function with a db object
//this new function was generated by sqlc in db.go, it creates and returns query object

func NewStore(db *sql.DB) *Store {
	return &Store{
		db:      db,
		Queries: New(db),
	}
}

// exect Tx executes a function within a database transaction

//it takes a contex and a function as input, then starts a new db transaction
func (store *Store) execTx(ctx context.Context, fn func(*Queries) error) error {
	//first put in begin text, and the default text option
	//if there is error in this step, call error immediately
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	//otherwise call a new function with crud transaction and get back new queries object
	//this new fnction pass a text object. now we have a query that runs in the transaction

	q := New(tx)
	// call the input function with query, and return error
	err = fn(q)
	//if the error is not nil, we call back the transaction.
	//tx.rollback returns a roll back error, if the rollback erroris not nil, we have to report 2 errors. so we should combine them into 1 error before returining
	//first the transaction error, then the rollback error. if there is no rollback error, return transaction err
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)

		}
		return err

	}
	//if there is no error at all ,commit the transaction, and then return its error
	return tx.Commit()
	//the function returns an error but starts with small case e and its not exported
}

//TransferTxParams contains all the input params of the transfer transaction needed
//define the transfertxparams for the following function
type TransferTxParams struct {
	//the account from which the money is transfered from
	FromAccountID int64 `json:"from_account_id"`
	ToAccountID   int64 `json:"to_account_id"` //the account to which to money is transfered to
	Amount        int64 `json:"amount"`
}

//TransferTxResult is the result of the transfer transaction. have 5 fileds
type TransferTxResult struct {
	Transfer    Transfer `json:"transfer"`     //the transfer action
	FromAccount Account  `json:"from_account"` //the from account after it's balance is updated
	ToAccount   Account  `json:"to_account"`   //the to account after it's balance is updated
	FromEntry   Entry    `json:"from_entry"`   //the entry of the from account that the money is moving out
	ToEntry     Entry    `json:"to_entry"`     //the entry of the to account that the money is moving in

}

var txKey = struct{}{}

//TransferTx performs a money transfer from one account to the other
//it creates a transfer record, add account entries, and update accounts' balance within a single database transaction
//it takes contex and transferpara as input, and returns transfertxrusulet and errpr

func (store *Store) TransferTx(ctx context.Context, arg TransferTxParams) (TransferTxResult, error) {
	var result TransferTxResult //create an empty resuilt
	// call the store.exectTx to create and run new db transaction, pass the context, and call back function
	err := store.execTx(ctx, func(q *Queries) error {
		var err error //need to declare this error

		txName := ctx.Value(txKey)

		fmt.Println(txName, "create transfer")

		result.Transfer, err = q.CreateTransfer(ctx, CreateTransferParams{
			FromAccountID: arg.FromAccountID,
			ToAccountID:   arg.ToAccountID,
			Amount:        arg.Amount,
		})
		if err != nil {
			return err //if error not nil return right away
		}

		fmt.Println(txName, "create entry 1")

		result.FromEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID: arg.FromAccountID,
			Amount:    -arg.Amount,
		})

		if err != nil {
			return err
		}

		fmt.Println(txName, "create entry 2")

		result.ToEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID: arg.ToAccountID,
			Amount:    arg.Amount,
		})
		if err != nil {
			return err
		}

		//TODO: UPdate accounts' balance
		// call getaccount to get the from account record

		//if there is no err, update account

		if arg.FromAccountID < arg.ToAccountID {
			//make sure that the toaccount is updated before the from account
			result.FromAccount, result.ToAccount, err = addMoney(ctx, q, arg.FromAccountID, -arg.Amount, arg.ToAccountID, arg.Amount)

		} else {
			result.ToAccount, result.FromAccount, err = addMoney(ctx, q, arg.ToAccountID, arg.Amount, arg.FromAccountID, -arg.Amount)
		}

		return nil
	})
	return result, err //return the result and err of the exct call

}

func addMoney(
	ctx context.Context,
	q *Queries,
	accountID1 int64,
	amount1 int64,
	accountID2 int64,
	amount2 int64,
) (account1 Account, account2 Account, err error) {
	account1, err = q.AddAccountBalance(ctx, AddAccountBalanceParams{
		ID:     accountID1,
		Amount: amount1,
	})
	if err != nil {
		return //it is the same with to return account1, account2 , err
	}

	account2, err = q.AddAccountBalance(ctx, AddAccountBalanceParams{
		ID:     accountID2,
		Amount: amount2,
	})
	return
}
