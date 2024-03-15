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

type accSvc struct {
	logger         *zap.SugaredLogger
	accDb          *gorm.DB
	authServer     string
	authHttpClient *http.Client
	kafkaProducer  *kafka.Producer
	kafkaConsumer  *kafka.Consumer
}

func main() {
	zapLogger := zap.New(common.GetZapCore(true))
	logger := zapLogger.Sugar()
	logger.Info("Starting aTES.Accounting service")

	webAddress := os.Getenv("ATES_ACC_SERVER")
	if webAddress == "" {
		webAddress = ":7002"
	}
	mysqlDsn := os.Getenv("ATES_ACC_MYSQL")
	if mysqlDsn == "" {
		logger.Fatalf("Missing mysql dsn string in ATES_ACC_MYSQL env")
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
		"group.id":          "Accounting",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		logger.Fatalf("Failed to initialize Kafka Consumer")
		os.Exit(-1)
	}

	err = kafkaConsumer.SubscribeTopics([]string{"user.lifecycle", "task.lifecycle"}, nil)
	if err != nil {
		logger.Fatalf("Failed to subscribe to necessary Kafka topics")
		os.Exit(-1)
	}

	kafkaProducer, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": "localhost"})
	if err != nil {
		logger.Fatalf("Failed to initialize Kafka Producer")
		os.Exit(-1)
	}
	defer kafkaProducer.Close()
	go func() {
		for e := range kafkaProducer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					logger.Errorf("Delivery failed: %v\n", ev.TopicPartition)
				} else {
					logger.Debugf("Delivered message to %v\n", ev.TopicPartition)
				}
			}
		}
	}()
	e := common.GetNewEcho(logger)
	e.Use(middleware.Recover())

	db, err := gorm.Open(mysql.Open(mysqlDsn), &gorm.Config{})
	if err != nil {
		logger.Fatalf("Failed to connect to mysql database with dsn provided in ATES_TM_MYSQL")
		os.Exit(-1)
	}

	// Ensure tables
	_ = db.AutoMigrate(&User{}, &Task{}, &BillingCycle{}, &Account{}, &OperationType{})
	//_ = db.AutoMigrate(&AccountLog{})
	//createDefaultOperations(db)

	// todo: find a way to create GORM table without key and constraint
	// alter table account_logs drop constraint fk_account_logs_task
	// alter table account_logs drop key fk_account_logs_task;

	app := accSvc{
		logger:     logger,
		accDb:      db,
		authServer: common.EnsureServerProtocol(authServer),
		authHttpClient: &http.Client{
			Transport: &http.Transport{
				//TLSClientConfig: tlsConfig,
			},
		},
		kafkaProducer: kafkaProducer,
		kafkaConsumer: kafkaConsumer,
	}

	e.GET("/log/my", app.getLog)
	e.GET("/log/:day", app.getLogOnDay)
	e.GET("/balance/my", app.getBalance)

	e.GET("/income/today", app.getIncome)
	e.GET("/income/:day", app.getIncomeOnDay)

	e.POST("/closeday", app.closeDay)

	abortReadCh := make(chan bool)
	go app.startReadingNotification(abortReadCh)

	e.Logger.Fatal(e.Start(webAddress))

	abortReadCh <- true
}
