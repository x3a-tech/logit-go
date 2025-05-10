package logit

import (
	"fmt"
	"gopkg.in/natefinch/lumberjack.v2"
	"sync"
	"time"
)

// TimeRotatingWriter обеспечивает запись с ротацией логов по времени,
// используя lumberjack.Logger для ротации по размеру/возрасту/количеству.
type TimeRotatingWriter struct {
	*lumberjack.Logger // Встраиваем lumberjack.Logger для его функциональности
	rotationTime       time.Duration
	lastRotation       time.Time
	mu                 sync.Mutex
}

// NewTimeRotatingWriter создает новый TimeRotatingWriter.
// logger - это экземпляр lumberjack.Logger.
// rotationTime - длительность, после которой будет произведена принудительная ротация,
// например, 24*time.Hour. Если rotationTime <= 0, ротация по времени не будет активна.
func NewTimeRotatingWriter(logger *lumberjack.Logger, rotationTime time.Duration) *TimeRotatingWriter {
	return &TimeRotatingWriter{
		Logger:       logger,
		rotationTime: rotationTime,
		lastRotation: time.Now(), // Устанавливаем время последней ротации на текущее
	}
}

// Write реализует интерфейс io.Writer.
// Он записывает данные в лог-файл и выполняет ротацию, если пришло время.
func (w *TimeRotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Проверяем, нужно ли ротировать файл по времени
	// Это условие должно быть истинным, только если rotationTime > 0
	if w.rotationTime > 0 && time.Since(w.lastRotation) >= w.rotationTime {
		// Выполняем ротацию через встроенный lumberjack.Logger
		// lumberjack.Rotate() сам обрабатывает переименование и т.д.
		if err := w.Logger.Rotate(); err != nil {
			// Если ротация не удалась, мы все равно пытаемся записать лог,
			// но возвращаем ошибку ротации.
			// Или можно вернуть 0, err немедленно.
			// Запись в старый файл может быть предпочтительнее потери логов.
			currentN, writeErr := w.Logger.Write(p)
			if writeErr != nil {
				return 0, fmt.Errorf("ошибка записи после ошибки ротации: %v (ошибка ротации: %v)", writeErr, err)
			}
			return currentN, fmt.Errorf("ошибка ротации лог-файла: %v", err)
		}
		w.lastRotation = time.Now() // Обновляем время последней ротации
	}

	// Записываем данные через встроенный lumberjack.Logger
	return w.Logger.Write(p)
}

// Sync реализует zapcore.WriteSyncer, если это необходимо (хотя zapcore.AddSync обычно используется).
// Для lumberjack.Logger Sync не определен, но он реализует io.WriteCloser.
// Если вам нужен Sync, его можно добавить так, или убедиться, что zapcore.AddSync используется правильно.
// func (w *TimeRotatingWriter) Sync() error {
//	 // lumberjack.Logger не имеет метода Sync().
//	 // Если нижележащий файл является os.File, можно было бы вызвать его Sync().
//	 // В данном случае, можно либо ничего не делать, либо вернуть nil.
//	 return nil
// }

// Close закрывает логгер. Реализует io.Closer.
func (w *TimeRotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Logger.Close()
}
