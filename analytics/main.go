package main

import (
	"ates/common"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"net/http"
	"os"
)

type anSvc struct {
	logger         *zap.SugaredLogger
	anDb           *gorm.DB
	authServer     string
	authHttpClient *http.Client
	kafkaConsumer  *kafka.Consumer
}

func main() {
	zapLogger := zap.New(common.GetZapCore(true))
	logger := zapLogger.Sugar()
	logger.Info("Starting aTES.Analytics service")

	webAddress := os.Getenv("ATES_AN_SERVER")
	if webAddress == "" {
		webAddress = ":7003"
	}
	mysqlDsn := os.Getenv("ATES_AN_MYSQL")
	if mysqlDsn == "" {
		logger.Fatalf("Missing mysql dsn string in ATES_AN_MYSQL env")
		os.Exit(-1)
	}
	authServer := os.Getenv("ATES_AUTH_SERVER")
	if authServer == "" {
		logger.Fatalf("Missing address of Auth server in ATES_AUTH_SERVER env")
		os.Exit(-1)
	}
	kafkaAddress := os.Getenv("ATES_KAFKA")
	if kafkaAddress == "" {
		logger.Fatalf("Missing kafka address in ATES_KAFKA env")
		os.Exit(-1)
	}

	kafkaConsumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": "localhost",
		"group.id":          "Analytics",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		logger.Fatalf("Failed to initialize Kafka Consumer")
		os.Exit(-1)
	}

	err = kafkaConsumer.SubscribeTopics([]string{"user.lifecycle", "accountlog.lifecycle"}, nil)
	if err != nil {
		logger.Fatalf("Failed to subscribe to necessary Kafka topics")
		os.Exit(-1)
	}

	e := common.GetNewEcho(logger)
	e.Use(middleware.Recover())

	db, err := gorm.Open(mysql.Open(mysqlDsn), &gorm.Config{})
	if err != nil {
		logger.Fatalf("Failed to connect to mysql database with dsn provided in ATES_TM_MYSQL")
		os.Exit(-1)
	}

	// Ensure tables
	_ = db.AutoMigrate(&User{}, &AccountLog{})

	app := anSvc{
		logger:     logger,
		anDb:       db,
		authServer: common.EnsureServerProtocol(authServer),
		authHttpClient: &http.Client{
			Transport: &http.Transport{
				//TLSClientConfig: tlsConfig,
			},
		},
		kafkaConsumer: kafkaConsumer,
	}

	e.GET("/analytics/today", nil)

	abortReadCh := make(chan bool)
	go app.startReadingNotification(abortReadCh)

	e.Logger.Fatal(e.Start(webAddress))

	abortReadCh <- true
}
