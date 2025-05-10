package logit

import (
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/x3a-tech/configo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type contextKey string

const (
	opKey      contextKey = "op"
	traceIDKey contextKey = "traceId"
)

type logIt struct {
	logger *zap.Logger
}

// Logger определяет интерфейс для логгера.
type Logger interface {
	Debug(ctx context.Context, message string, fields ...zap.Field) // Изменена сигнатура
	Info(ctx context.Context, message string, fields ...zap.Field)
	Infof(ctx context.Context, message string, a ...any)
	Warn(ctx context.Context, message string, fields ...zap.Field)
	Warnf(ctx context.Context, message string, a ...any)
	Error(ctx context.Context, err error, fields ...zap.Field)
	Errorf(ctx context.Context, format string, args ...interface{})
	Fatal(ctx context.Context, err error, fields ...zap.Field)
	Fatalf(ctx context.Context, format string, args ...interface{})
	NewCtx(ctx context.Context, op string, traceID *string) context.Context
	NewOpCtx(ctx context.Context, op string) context.Context
	NewTraceCtx(ctx context.Context, traceID *string) context.Context
	NewTraceContext(traceID *string) context.Context
}

// Params содержит параметры для инициализации логгера.
type Params struct {
	AppConf    *configo.App
	LoggerConf *configo.Logger
	SenConf    *configo.Sentry
	Env        *configo.Env // Убедитесь, что configo.Env существует и имеет метод IsLocal()
}

// MustNewLogger создает новый экземпляр Logger.
// Паникует, если конфигурация некорректна или отсутствуют обязательные параметры.
func MustNewLogger(params *Params) Logger {
	if params == nil {
		panic("logger: параметры инициализации (params) не могут быть nil")
	}
	if params.AppConf == nil {
		panic("logger: конфигурация приложения (AppConf) не может быть nil")
	}
	if params.LoggerConf == nil {
		panic("logger: конфигурация логгера (LoggerConf) не может быть nil")
	}
	if params.Env == nil {
		panic("logger: конфигурация окружения (Env) не может быть nil")
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace", // Ключ для стектрейса
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder, // Цветной вывод уровня для консоли
		EncodeTime:     zapcore.TimeEncoderOfLayout(params.LoggerConf.TimeFormat),
		EncodeDuration: zapcore.StringDurationEncoder,
		// EncodeCaller:   zapcore.ShortCallerEncoder, // Раскомментируйте, если нужен вывод места вызова. Потребуется zap.AddCaller() и возможно zap.AddCallerSkip().
	}

	var encoder zapcore.Encoder

	if !params.Env.IsLocal() {
		if params.SenConf != nil && params.SenConf.Key != "" && params.SenConf.Host != "" {
			// Sentry DSN формат: "https://<key>@<host>/<project_id>"
			// Убедитесь, что params.SenConf.Host это домен (например, sentry.example.com)
			// и если нужен ID проекта, он должен быть частью Host или добавлен отдельно.
			// Текущая строка: fmt.Sprintf("https://%s@%s", params.SenConf.Key, params.SenConf.Host)
			// может потребовать корректировки в зависимости от вашей конфигурации Sentry.
			err := sentry.Init(sentry.ClientOptions{
				Dsn:              fmt.Sprintf("https://%s@%s", params.SenConf.Key, params.SenConf.Host),
				TracesSampleRate: 1.0,                  // Отправлять 100% трейсов, настройте по необходимости
				Debug:            params.Env.IsLocal(), // Включать Debug для Sentry только в локальном окружении
				Environment:      params.Env.String(),
				Release:          fmt.Sprintf("%s@%s", params.AppConf.Name, params.AppConf.Version),
			})

			if err != nil {
				// Вместо паники можно логировать ошибку стандартным логгером и продолжить без Sentry
				fmt.Fprintf(os.Stderr, "Ошибка инициализации Sentry: %v\n", err)
				// panic("Ошибка инициализации Sentry: " + err.Error()) // Или оставить панику, если Sentry критичен
			}
		}
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder // Для JSON логов цвета не нужны
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig) // Цветной вывод для локальной консоли
	}

	var cores []zapcore.Core

	if params.LoggerConf.EnableConsole {
		consoleWriter := zapcore.Lock(os.Stdout)
		cores = append(cores, zapcore.NewCore(encoder, consoleWriter, zapcore.Level(params.LoggerConf.ConsoleLevel)))
	}

	if params.LoggerConf.EnableFile {
		rotationTime, err := time.ParseDuration(params.LoggerConf.RotationTime)
		if err != nil {
			panic("Некорректное время ротации (RotationTime): " + err.Error() + ". Используйте формат вроде '24h', '5m'.")
		}

		// Имя файла для lumberjack. Lumberjack будет ротировать этот файл.
		// Если fileName включает дату, то каждый день будет создаваться новый *базовый* файл,
		// и lumberjack будет ротировать уже его.
		// TimeRotatingWriter затем будет принудительно ротировать этот файл по времени.
		// Если нужен один непрерывный лог-файл, ротируемый по времени и размеру,
		// то дата в fileName может быть избыточной.
		// Текущая реализация fileName создает новый файл каждый день.
		logFilePath := filepath.Join(params.LoggerConf.Dir, fileName(params.AppConf.Name, params.AppConf.Version))

		lumberjackLogger := &lumberjack.Logger{
			Filename:   logFilePath,
			MaxSize:    params.LoggerConf.MaxSize, // в мегабайтах
			MaxBackups: params.LoggerConf.MaxBackups,
			MaxAge:     params.LoggerConf.MaxAge, // в днях
			Compress:   params.LoggerConf.Compress,
		}

		var writer zapcore.WriteSyncer
		if rotationTime > 0 {
			timeRotatingWriter := NewTimeRotatingWriter(lumberjackLogger, rotationTime)
			writer = zapcore.AddSync(timeRotatingWriter)
		} else {
			writer = zapcore.AddSync(lumberjackLogger) // Используем только lumberjack если rotationTime не задан (или 0)
		}
		cores = append(cores, zapcore.NewCore(encoder, writer, zapcore.Level(params.LoggerConf.FileLevel)))
	}

	if len(cores) == 0 {
		// Если не включен ни консольный, ни файловый вывод, логирование не будет работать.
		// Можно либо запаниковать, либо по умолчанию включить консольный вывод.
		// Здесь мы создадим Nop-логгер, чтобы приложение не падало, но логи не писались.
		// Либо можно добавить ядро по умолчанию:
		// consoleWriter := zapcore.Lock(os.Stdout)
		// cores = append(cores, zapcore.NewCore(encoder, consoleWriter, zapcore.InfoLevel))
		// fmt.Fprintln(os.Stderr, "Внимание: логирование не настроено (ни консоль, ни файл), используется NopLogger.")
		// return NewNopLogger()
		// Пока оставим панику, если нет ядер, это явная ошибка конфигурации.
		if !params.LoggerConf.EnableConsole && !params.LoggerConf.EnableFile {
			panic("logger: не включен ни консольный, ни файловый вывод логов.")
		}
	}

	core := zapcore.NewTee(cores...)

	// zap.AddCaller() - добавляет информацию о файле и строке вызова.
	// zap.AddCallerSkip(1) - если обертка логгера состоит из одного уровня,
	// нужно пропустить 1 уровень стека, чтобы показать реальное место вызова.
	// Настройте skipCount, если у вас несколько уровней оберток.
	// logger := zap.New(core, zap.AddStacktrace(zapcore.ErrorLevel), zap.AddCaller(), zap.AddCallerSkip(1))
	logger := zap.New(core, zap.AddStacktrace(zapcore.ErrorLevel)) // Добавляем AddCaller и AddCallerSkip при необходимости

	// Добавляем стандартные поля
	fields := []zap.Field{
		zap.String("appName", params.AppConf.Name),
		zap.String("appVersion", params.AppConf.Version),
		// Можно использовать zap.Namespace для группировки:
		// zap.Namespace("app"),
		// zap.String("name", params.AppConf.Name),
		// zap.String("version", params.AppConf.Version),
	}

	logger = logger.With(fields...)

	return &logIt{logger: logger}
}

// NewNopLogger создает логгер, который ничего не делает. Полезен для тестов.
func NewNopLogger() Logger {
	nopCore := zapcore.NewNopCore()
	nopLogger := zap.New(nopCore)
	return &logIt{logger: nopLogger}
}

// Debug - логирование отладочной информации (структурированное)
func (l *logIt) Debug(ctx context.Context, message string, fields ...zap.Field) {
	op := l.getOpFromContext(ctx)
	traceID := l.getTraceIDFromContext(ctx)
	allFields := append([]zap.Field{
		zap.String(string(opKey), op),
		zap.String(string(traceIDKey), traceID),
	}, fields...)
	l.logger.Debug(message, allFields...)
}

func (l *logIt) Info(ctx context.Context, message string, fields ...zap.Field) {
	op := l.getOpFromContext(ctx)
	traceID := l.getTraceIDFromContext(ctx)
	l.logger.Info(
		message,
		append([]zap.Field{
			zap.String(string(opKey), op),
			zap.String(string(traceIDKey), traceID),
		}, fields...)...,
	)
}

func (l *logIt) Infof(ctx context.Context, message string, a ...any) {
	op := l.getOpFromContext(ctx)
	traceID := l.getTraceIDFromContext(ctx)
	l.logger.Info(
		fmt.Sprintf(message, a...),
		zap.String(string(opKey), op),
		zap.String(string(traceIDKey), traceID),
	)
}

func (l *logIt) Warn(ctx context.Context, message string, fields ...zap.Field) {
	op := l.getOpFromContext(ctx)
	traceID := l.getTraceIDFromContext(ctx)
	l.logger.Warn(
		message,
		append([]zap.Field{
			zap.String(string(opKey), op),
			zap.String(string(traceIDKey), traceID),
		}, fields...)...,
	)
}

func (l *logIt) Warnf(ctx context.Context, message string, a ...any) {
	op := l.getOpFromContext(ctx)
	traceID := l.getTraceIDFromContext(ctx)
	l.logger.Warn(
		fmt.Sprintf(message, a...),
		zap.String(string(opKey), op),
		zap.String(string(traceIDKey), traceID),
	)
}

func (l *logIt) Error(ctx context.Context, err error, fields ...zap.Field) {
	op := l.getOpFromContext(ctx)
	traceID := l.getTraceIDFromContext(ctx)
	// Добавляем само сообщение об ошибке как поле, если его нет в fields
	// Zap автоматически добавляет стектрейс для ErrorLevel и выше, если настроено AddStacktrace.
	l.logger.Error(
		err.Error(), // Сообщение ошибки
		append([]zap.Field{
			zap.String(string(opKey), op),
			zap.String(string(traceIDKey), traceID),
			zap.Error(err), // Добавляем саму ошибку как структурированное поле
		}, fields...)...,
	)
	sentry.CaptureException(err) // Отправляем ошибку в Sentry
}

func (l *logIt) Errorf(ctx context.Context, format string, args ...interface{}) {
	op := l.getOpFromContext(ctx)
	traceID := l.getTraceIDFromContext(ctx)
	err := fmt.Errorf(format, args...)
	l.logger.Error(
		err.Error(),
		zap.String(string(opKey), op),
		zap.String(string(traceIDKey), traceID),
		zap.Error(err),
	)
	sentry.CaptureException(err)
}

func (l *logIt) Fatal(ctx context.Context, err error, fields ...zap.Field) {
	op := l.getOpFromContext(ctx)
	traceID := l.getTraceIDFromContext(ctx)
	l.logger.Fatal(
		err.Error(),
		append([]zap.Field{
			zap.String(string(opKey), op),
			zap.String(string(traceIDKey), traceID),
			zap.Error(err),
		}, fields...)...,
	)
	// Sentry.CaptureException(err) здесь не нужен, т.к. Fatal завершит программу,
	// и Sentry SDK обычно перехватывает паники/фатальные ошибки, если настроен.
	// Однако, для явности или если есть специфические требования к flush Sentry перед выходом,
	// можно оставить или использовать sentry.Flush().
	// sentry.CaptureException(err)
	// sentry.Flush(2 * time.Second) // Дать Sentry время на отправку
}

func (l *logIt) Fatalf(ctx context.Context, format string, args ...interface{}) {
	op := l.getOpFromContext(ctx)
	traceID := l.getTraceIDFromContext(ctx)
	err := fmt.Errorf(format, args...)
	l.logger.Fatal(
		err.Error(),
		zap.String(string(opKey), op),
		zap.String(string(traceIDKey), traceID),
		zap.Error(err),
	)
	// sentry.CaptureException(err)
	// sentry.Flush(2 * time.Second)
}

// NewCtx создает новый контекст с указанной операцией и traceId.
// Если ctx равен nil, используется context.Background().
func (l *logIt) NewCtx(ctx context.Context, op string, traceID *string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	var currentTraceID string
	if traceID != nil {
		currentTraceID = *traceID
	} else {
		// Если traceId не был передан, попытаемся извлечь его из существующего контекста
		existingTraceID, ok := ctx.Value(traceIDKey).(string)
		if ok && existingTraceID != "" {
			currentTraceID = existingTraceID
		} else {
			currentTraceID = uuid.New().String() // Генерируем новый, если нет
		}
	}
	ctx = context.WithValue(ctx, opKey, op)
	return context.WithValue(ctx, traceIDKey, currentTraceID)
}

// NewOpCtx создает новый контекст с указанной операцией.
// Если ctx равен nil, используется context.Background().
func (l *logIt) NewOpCtx(ctx context.Context, op string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, opKey, op)
}

// NewTraceCtx создает новый контекст с указанным traceId.
// Если ctx равен nil, используется context.Background().
// Если traceID равен nil, генерируется новый UUID.
func (l *logIt) NewTraceCtx(ctx context.Context, traceID *string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	var currentTraceID string
	if traceID != nil {
		currentTraceID = *traceID
	} else {
		currentTraceID = uuid.New().String()
	}
	return context.WithValue(ctx, traceIDKey, currentTraceID)
}

// NewTraceContext создает новый корневой контекст с указанным traceId.
// Если traceID равен nil, генерируется новый UUID.
func (l *logIt) NewTraceContext(traceID *string) context.Context {
	var currentTraceID string
	if traceID != nil {
		currentTraceID = *traceID
	} else {
		currentTraceID = uuid.New().String()
	}
	return context.WithValue(context.Background(), traceIDKey, currentTraceID)
}

// getOpFromContext извлекает операцию (op) из контекста.
func (l *logIt) getOpFromContext(ctx context.Context) string {
	if ctx == nil {
		return "unknown_op_nil_context"
	}
	if op, ok := ctx.Value(opKey).(string); ok {
		return op
	}
	return "unknown_op"
}

// getTraceIDFromContext извлекает идентификатор трассировки (traceId) из контекста.
// Если traceId не найден, генерируется новый UUID.
func (l *logIt) getTraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return uuid.New().String() // Генерируем новый для nil контекста
	}
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	return uuid.New().String() // Генерируем новый, если не найден
}

// fileName генерирует имя файла лога на основе имени приложения, версии и текущей даты.
// Формат: appName_appVersion_YYYY-MM-DD.log
func fileName(appName, appVersion string) string {
	// Замените недопустимые символы в имени и версии, если они могут там быть
	safeAppName := strings.ReplaceAll(appName, "/", "_")
	safeAppVersion := strings.ReplaceAll(appVersion, "/", "_")
	currentDate := time.Now().Format("2006-01-02")
	return fmt.Sprintf("%s_%s_%s.log", safeAppName, safeAppVersion, currentDate)
}
