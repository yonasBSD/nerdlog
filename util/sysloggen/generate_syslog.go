package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

var facilities = []string{
	"kern", "user", "mail", "daemon", "auth", "syslog",
	"lpr", "news", "uucp", "cron", "authpriv", "ftp",
}

var severities = []string{
	"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug",
}

var messages = []string{
	"Connection established",
	"User login successful",
	"Disk space low",
	"System rebooted",
	"Configuration updated",
	"Unauthorized access attempt",
	"Service started",
	"File not found",
	"Network interface down",
	"Memory usage high",
	"Process terminated",
	"CPU temperature critical",
	"New device connected",
	"Package installation completed",
	"Database query failed",
	"Service stopped",
	"Error reading file",
	"Permission denied",
	"Disk write error",
	"Failed login attempt",
	"Configuration load failed",
	"Authentication failure",
	"Server shutting down",
	"Session expired",
	"Process started",
	"Timeout occurred",
	"Network unreachable",
	"Port unreachable",
	"Insufficient privileges",
	"DNS resolution failed",
	"Database connection error",
	"Configuration reload successful",
	"File system full",
	"System performance degraded",
	"Kernel panic",
	"Backup completed",
	"Backup failed",
	"File copied successfully",
	"Process crashed",
	"Memory leak detected",
	"New update available",
	"Update failed",
	"Service restart requested",
	"Database schema updated",
	"Invalid input detected",
	"Request timed out",
	"Resource allocation failed",
	"Firewall rule added",
	"Firewall rule deleted",
	"User session started",
	"User session ended",
	"Disk format completed",
	"SSH connection established",
	"SSH connection closed",
	"Unexpected error occurred",
	"Request successfully processed",
	"Service restart completed",
	"Cache cleared",
	"Cache update completed",
	"Configuration applied successfully",
	"Service health check failed",
	"System running low on resources",
	"File system check completed",
	"Software version updated",
	"Memory usage normal",
	"Disk usage critical",
	"Network congestion detected",
	"Network speed reduced",
	"Service unavailable",
	"Hardware failure detected",
	"Service dependency failure",
	"System time updated",
	"Invalid credentials provided",
	"Login attempt locked out",
	"User account disabled",
	"User account enabled",
	"Service request queued",
	"Service request completed",
	"User permissions updated",
	"Certificate expiration warning",
	"Application crash reported",
	"Security patch applied",
	"Backup restoration completed",
	"Maintenance mode enabled",
	"Maintenance mode disabled",
	"Security alert raised",
	"Security breach detected",
	"Resource utilization warning",
	"High CPU usage detected",
	"High memory usage detected",
	"Disk error occurred",
	"Data corruption detected",
	"API request failed",
	"API response received",
	"File checksum mismatch",
	"Hardware upgrade completed",
	"System configuration backed up",
	"User session timed out",
	"System configuration restored",
	"Service initialization failed",
	"Server started successfully",
	"Server stopped unexpectedly",
	"Logging level changed",
	"Error handling request",
	"Out of memory error",
	"Service dependency initialized",
	"Scheduled task executed",
	"Scheduled task failed",
	"User authentication successful",
	"User authentication failed",
	"Session token expired",
	"File upload completed",
	"File download started",
	"File upload failed",
	"File download failed",
	"File transfer completed",
	"File transfer failed",
	"System clock synchronized",
	"System time drift detected",
	"Software upgrade completed",
	"System reboot required",
	"Disk space reclaimed",
	"IP address conflict detected",
	"Application configuration error",
	"System health check completed",
	"System health check failed",
	"Log file rotated",
	"Log file archived",
	"User password changed",
	"Invalid password attempt",
	"SMTP server connection error",
	"Database migration completed",
	"Database migration failed",
	"Network link restored",
	"Network interface reset",
}

func randomElement(list []string) string {
	return list[rand.Intn(len(list))]
}

func generateSyslogEntry(ts time.Time) string {
	timestamp := ts.Format("Jan _2 15:04:05")
	hostname := "myhost"
	tag := randomElement(facilities)
	severity := randomElement(severities)
	pid := rand.Intn(9000) + 100
	message := randomElement(messages)

	return fmt.Sprintf("%s %s %s[%d]: <%s> %s", timestamp, hostname, tag, pid, severity, message)
}

func randomStep() time.Duration {
	if rand.Float64() < 0.2 {
		// 2% chance of 1ms burst
		return 1 * time.Millisecond
	}
	// Otherwise, pick between 2ms and 10min
	min := int64(2 * time.Millisecond)
	max := int64(10 * time.Minute)
	return time.Duration(rand.Int63n(max-min) + min)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	file, err := os.Create("random_syslog.log")
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	curTime, err := time.Parse(time.RFC3339, "2025-03-09T15:04:05Z")
	if err != nil {
		panic(err.Error())
	}

	for i := 0; i < 1000; i++ {
		curTime = curTime.Add(randomStep())
		logEntry := generateSyslogEntry(curTime)
		_, err := file.WriteString(logEntry + "\n")
		if err != nil {
			fmt.Println("Error writing to file:", err)
			return
		}
	}

	fmt.Println("Generated 1000 syslog entries with random steps and 2% 1ms bursts.")
}
