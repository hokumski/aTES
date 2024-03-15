package main

import (
	"ates/common"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-oauth2/oauth2/v4/store"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"os"
)

type authSvc struct {
	logger        *zap.SugaredLogger
	oauthServer   *server.Server
	userDb        *gorm.DB
	kafkaProducer *kafka.Producer
}

func main() {

	zapLogger := zap.New(common.GetZapCore(true))
	logger := zapLogger.Sugar()
	logger.Info("Starting aTES.Auth service")

	webAddress := os.Getenv("ATES_AUTH_SERVER")
	if webAddress == "" {
		webAddress = ":7000"
	}
	mysqlDsn := os.Getenv("ATES_AUTH_MYSQL")
	if mysqlDsn == "" {
		logger.Fatalf("Missing mysql dsn string in ATES_AUTH_MYSQL env")
		os.Exit(-1)
	}

	kafkaAddress := os.Getenv("ATES_KAFKA")
	if kafkaAddress == "" {
		logger.Fatalf("Missing kafka address in ATES_KAFKA env")
		os.Exit(-1)
	}

	e := common.GetNewEcho(logger)
	e.Use(middleware.Recover())

	db, err := gorm.Open(mysql.Open(mysqlDsn), &gorm.Config{})
	if err != nil {
		logger.Fatalf("Failed to connect to mysql database with dsn provided in ATES_AUTH_MYSQL")
		os.Exit(-1)
	}

	// Ensure tables
	_ = db.AutoMigrate(&User{}, &Role{})
	createDefaultRoles(db)

	clientStore := NewClientStore(db)

	manager := manage.NewDefaultManager()
	manager.SetAuthorizeCodeTokenCfg(manage.DefaultAuthorizeCodeTokenCfg)
	manager.MapClientStorage(clientStore)

	// Will store access tokens in memory
	manager.MustTokenStorage(store.NewMemoryTokenStore())

	srv := server.NewDefaultServer(manager)
	srv.SetAllowGetAccessRequest(true)
	srv.SetClientInfoHandler(server.ClientFormHandler)

	srv.SetInternalErrorHandler(func(err error) (re *errors.Response) {
		logger.Errorf("Internal Error: %s", err.Error())
		return
	})
	srv.SetResponseErrorHandler(func(re *errors.Response) {
		logger.Errorf("Response Error: %s", re.Error.Error())
	})

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

	app := authSvc{
		logger:        logger,
		oauthServer:   srv,
		userDb:        db,
		kafkaProducer: kafkaProducer,
	}
	srv.SetPasswordAuthorizationHandler(app.checkPassword)

	// At this time we support only token requests with Password flow (provide username/password)
	// Not secure enough

	e.GET("/oauth/token", app.token)
	//e.GET("oauth/authorize", app.authorize)
	e.POST("/register", app.registerUser)
	e.GET("/verify", app.verify)
	e.POST("/verify", app.verify)

	e.Logger.Fatal(e.Start(webAddress))
}
