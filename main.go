package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Customers struct {
	CustomerNumber int    `json:"customer_number" gorm:"primaryKey:auto_increment"`
	Name           string `json:"name"`
}

type Accounts struct {
	AccountNumber  int `gorm:"primaryKey:auto_increment"`
	CustomerNumber int `json:"customer_number"`
	Balance        int `json:"balance"`
}

func main() {
	// Inisialisasi Echo
	e := echo.New()

	// conect database
	db, err := gorm.Open(postgres.Open("host=localhost user=postgres password=1234 dbname=db_customer port=5432"), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to database")
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic("Failed to initialize database connection pool")
	}
	defer sqlDB.Close()

	db.AutoMigrate(
		&Accounts{},
		&Customers{},
	)
	fmt.Println("migrasi sukses")

	// Endpoint untuk insert data tb_customer
	e.POST("/customer", func(c echo.Context) error {
		var customer Customers
		if err := c.Bind(&customer); err != nil {
			return c.JSON(http.StatusBadRequest, "Data customer tidak valid")
		}
		customer.CustomerNumber = int(time.Now().Unix())

		db.Create(&customer)

		return c.JSON(http.StatusCreated, &customer)
	})

	//tup-up balanca
	e.POST("/account", func(c echo.Context) error {
		var account Accounts
		if err := c.Bind(&account); err != nil {
			return c.JSON(http.StatusBadRequest, "Data account tidak valid")
		}

		// Cek apakah customer_number ada di tabel customers
		var customer Customers
		sql := `SELECT * FROM customers WHERE customer_number = ?`
		if err := db.Raw(sql, account.CustomerNumber).Scan(&customer).Error; err != nil {
			return c.JSON(http.StatusBadRequest, "Customer tidak ditemukan")
		}

		account.AccountNumber = int(time.Now().Unix())

		db.Create(&account)

		return c.JSON(http.StatusCreated, account)
	})

	// Endpoint untuk check saldo
	e.GET("/account/:account_number", func(c echo.Context) error {
		accountNumber := c.Param("account_number")
		account := Accounts{}
		if err := db.Where("account_number = ?", accountNumber).First(&account).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Account_number tidak ditemukan"})
		}
		customer := Customers{}
		if err := db.Where("customer_number = ?", account.CustomerNumber).First(&customer).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "customer_number tidak ditemukan"})
		}
		response := map[string]interface{}{
			"account_number": account.AccountNumber,
			"customer_name":  customer.Name,
			"balance":        account.Balance,
		}
		return c.JSON(http.StatusOK, response)
	})

	// Endpoint untuk transfer saldo
	e.POST("/account/:from_account_number/transfer", func(c echo.Context) error {
		fromAccountNumber := c.Param("from_account_number")
		var transferRequest struct {
			ToAccountNumber int `json:"to_account_number"`
			Amount          int `json:"amount"`
		}
		if err := c.Bind(&transferRequest); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}

		fromAccount := Accounts{}
		if err := db.Where("account_number = ?", fromAccountNumber).First(&fromAccount).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "akun sumber tidak ditemukan"})
		}

		toAccount := Accounts{}
		if err := db.Where("account_number = ?", transferRequest.ToAccountNumber).First(&toAccount).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "akun tujuan tidak ditemukan"})
		}

		if fromAccount.Balance < transferRequest.Amount {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "saldo tidak mencukupi"})
		}

		// Mulai transaksi
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// Update saldo akun
		fromAccount.Balance -= transferRequest.Amount
		toAccount.Balance += transferRequest.Amount

		if err := tx.Save(&fromAccount).Error; err != nil {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "gagal update saldo akun sumber"})
		}

		if err := tx.Save(&toAccount).Error; err != nil {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "gagal update saldo akun tujuan"})
		}

		// Commit transaksi
		if err := tx.Commit().Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "gagal comit transaksi"})
		}

		return c.NoContent(http.StatusCreated)
	})

	// Start server
	e.Logger.Fatal(e.Start("localhost:8000"))
}
