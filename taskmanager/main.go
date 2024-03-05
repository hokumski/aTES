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

type tmSvc struct {
	logger         *zap.SugaredLogger
	tmDb           *gorm.DB
	authServer     string
	authHttpClient *http.Client
	kafkaProducer  *kafka.Producer
	kafkaConsumer  *kafka.Consumer
}

func main() {
	zapLogger := zap.New(common.GetZapCore(true))
	logger := zapLogger.Sugar()
	logger.Info("Starting aTES.TaskManager service")

	webAddress := os.Getenv("ATES_TM_SERVER")
	if webAddress == "" {
		webAddress = ":7001"
	}
	mysqlDsn := os.Getenv("ATES_TM_MYSQL")
	if mysqlDsn == "" {
		logger.Fatalf("Missing mysql dsn string in ATES_TM_MYSQL env")
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

	kafkaConsumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": "localhost",
		"group.id":          "TaskManager",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		logger.Fatalf("Failed to initialize Kafka Consumer")
		os.Exit(-1)
	}

	err = kafkaConsumer.SubscribeTopics([]string{"user_events"}, nil)
	if err != nil {
		logger.Fatalf("Failed to subscribe to necessary Kafka topic")
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
	_ = db.AutoMigrate(&User{}, &Task{}, &Status{}, &TaskLog{})
	//createDefaultStatuses(db)

	app := tmSvc{
		logger:     logger,
		tmDb:       db,
		authServer: common.EnsureServerProtocol(authServer),
		authHttpClient: &http.Client{
			Transport: &http.Transport{
				//TLSClientConfig: tlsConfig,
			},
		},
		kafkaProducer: kafkaProducer,
		kafkaConsumer: kafkaConsumer,
	}

	e.POST("/tasks/new", app.newTask)
	e.POST("/tasks/reassign", app.reassignTasks)
	e.GET("/tasks/list", app.getOpenTasks)
	e.GET("/tasks/:tid", app.getTask)                // tid is UUID
	e.POST("/tasks/:tid/complete", app.completeTask) // tid is UUID

	abortReadCh := make(chan bool)
	go app.startReadingNotification(abortReadCh)

	e.Logger.Fatal(e.Start(webAddress))

	abortReadCh <- true
}
