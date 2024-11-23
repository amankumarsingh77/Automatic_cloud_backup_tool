package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)


type ExponentialBackoff struct {
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	MaxRetries    int
	BackoffFactor float64
	Jitter        float64
}


func NewExponentialBackoff() *ExponentialBackoff {
	return &ExponentialBackoff{
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Minute,
		MaxRetries:    5,
		BackoffFactor: 2.0,
		Jitter:        0.1,
	}
}


func (eb *ExponentialBackoff) WithMaxRetries(maxRetries int) *ExponentialBackoff {
	eb.MaxRetries = maxRetries
	return eb
}


func (eb *ExponentialBackoff) WithInitialDelay(delay time.Duration) *ExponentialBackoff {
	eb.InitialDelay = delay
	return eb
}


func (eb *ExponentialBackoff) WithMaxDelay(delay time.Duration) *ExponentialBackoff {
	eb.MaxDelay = delay
	return eb
}


func (eb *ExponentialBackoff) calculateDelay(attempt int) time.Duration {
	
	delayFloat := float64(eb.InitialDelay) * math.Pow(eb.BackoffFactor, float64(attempt))

	
	jitterRange := delayFloat * eb.Jitter
	jitter := (rand.Float64()*2 - 1) * jitterRange 

	delay := time.Duration(delayFloat + jitter)

	
	if delay > eb.MaxDelay {
		delay = eb.MaxDelay
	}

	return delay
}


type RetryableError struct {
	err       error
	retryable bool
}

func (e *RetryableError) Error() string {
	return e.err.Error()
}


func (e *RetryableError) IsRetryable() bool {
	return e.retryable
}


func NewRetryableError(err error, retryable bool) *RetryableError {
	return &RetryableError{
		err:       err,
		retryable: retryable,
	}
}


func (eb *ExponentialBackoff) RetryWithBackoff(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt < eb.MaxRetries; attempt++ {
		
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		default:
		}

		
		err := operation()
		if err == nil {
			return nil 
		}

		
		if retryErr, ok := err.(*RetryableError); ok && !retryErr.IsRetryable() {
			return err 
		}

		lastErr = err

		
		if attempt == eb.MaxRetries-1 {
			break
		}

		
		delay := eb.calculateDelay(attempt)
		timer := time.NewTimer(delay)

		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("operation cancelled during backoff: %w", ctx.Err())
		case <-timer.C:
			
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", eb.MaxRetries, lastErr)
}


func IsTemporaryError(err error) bool {
	if retryErr, ok := err.(*RetryableError); ok {
		return retryErr.IsRetryable()
	}

	
	
	return false
}
