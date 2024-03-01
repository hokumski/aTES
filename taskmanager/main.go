package main

import (
	"ates/common"
	"github.com/labstack/echo/v4/middleware"
	"github.com/segmentio/kafka-go"
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
	kafkaWriter    *kafka.Writer
	kafkaReader    *kafka.Reader
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

	kafkaWriter := &kafka.Writer{
		Addr:     kafka.TCP(kafkaAddress),
		Topic:    "task_events",
		Balancer: &kafka.LeastBytes{},
	}
	kafkaWriter.AllowAutoTopicCreation = true
	defer func(kafkaWriter *kafka.Writer) {
		_ = kafkaWriter.Close()
	}(kafkaWriter)

	readerConfig := kafka.ReaderConfig{
		Brokers:     []string{kafkaAddress},
		GroupID:     "TaskManager",
		GroupTopics: []string{"user_events"},
	}
	kafkaReader := kafka.NewReader(readerConfig)

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
		kafkaReader: kafkaReader,
		kafkaWriter: kafkaWriter,
	}

	e.POST("/tasks/new", app.newTask)
	e.POST("/tasks/reassign", app.reassignTasks)
	e.GET("/tasks/list", app.getOpenTasks)
	e.GET("/tasks/:tid", app.getTask)                // tid is UUID
	e.POST("/tasks/:tid/complete", app.completeTask) // tid is UUID

	abortReadCh := make(chan bool)
	abortProcessCh := make(chan bool)
	incomingNotificationsCh := make(chan Notification, 10)
	go app.startReadingNotification(incomingNotificationsCh, abortReadCh)
	go app.processNotifications(incomingNotificationsCh, abortProcessCh)

	e.Logger.Fatal(e.Start(webAddress))
}
