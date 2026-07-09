package user

import (
	"time"

	"github.com/dankedev/tulis-go/utils/mail"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// StartNotificationScheduler starts the background loop to check for user inactivity.
func StartNotificationScheduler(db *gorm.DB) {
	if db == nil {
		logrus.Warn("[SCHEDULER] Database connection is nil, background email scheduler not started")
		return
	}

	// Run check once on startup after 10 seconds, then every 24 hours
	go func() {
		logrus.Info("[SCHEDULER] Initializing background notification scheduler...")
		time.Sleep(10 * time.Second)
		for {
			logrus.Info("[SCHEDULER] Running background email inactivity checks...")
			check7DaysLoginInactivity(db)
			check30DaysWritingInactivity(db)
			
			// Wait for 24 hours before next check
			time.Sleep(24 * time.Hour)
		}
	}()
}

func check7DaysLoginInactivity(db *gorm.DB) {
	var users []User
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)

	// Query users who haven't logged in for 7 days and haven't been sent a reminder since last login
	err := db.Where(
		"(last_login_at < ? OR (last_login_at IS NULL AND created_at < ?)) AND (last_login_reminder_sent_at IS NULL OR (last_login_at IS NOT NULL AND last_login_reminder_sent_at < last_login_at))",
		sevenDaysAgo, sevenDaysAgo,
	).Find(&users).Error

	if err != nil {
		logrus.Errorf("[SCHEDULER] Error querying 7-day inactive users: %v", err)
		return
	}

	if len(users) == 0 {
		return
	}

	logrus.Infof("[SCHEDULER] Found %d users inactive for 7 days", len(users))
	now := time.Now()
	for _, u := range users {
		emailBody := mail.Get7DaysInactiveEmail(u.Name)
		err := mail.SendHTMLMail(u.Email, "Lama tidak berjumpa di Tulis CMS!", emailBody)
		if err == nil {
			u.LastLoginReminderSentAt = &now
			db.Save(&u)
		}
	}
}

func check30DaysWritingInactivity(db *gorm.DB) {
	var users []User
	thirtyDaysAgo := time.Now().Add(-30 * 24 * time.Hour)

	// We only check users who have roles that can write (superadmin, admin, editor, author)
	err := db.Where("role IN ?", []string{"superadmin", "admin", "editor", "author"}).Find(&users).Error
	if err != nil {
		logrus.Errorf("[SCHEDULER] Error querying authors for 30-day write check: %v", err)
		return
	}

	if len(users) == 0 {
		return
	}

	logrus.Infof("[SCHEDULER] Running 30-day writing check on %d users", len(users))
	now := time.Now()

	for _, u := range users {
		var latestPost struct {
			CreatedAt time.Time
		}
		// Query the user's latest created post
		err := db.Table("posts").
			Select("created_at").
			Where("author_id = ? AND deleted_at IS NULL", u.ID).
			Order("created_at DESC").
			Limit(1).
			Scan(&latestPost).Error

		if err != nil {
			continue
		}

		var lastWriteTime time.Time
		if latestPost.CreatedAt.IsZero() {
			// If never written, use registration time as last write reference
			lastWriteTime = u.CreatedAt
		} else {
			lastWriteTime = latestPost.CreatedAt
		}

		if lastWriteTime.Before(thirtyDaysAgo) {
			if u.LastWriteReminderSentAt == nil || u.LastWriteReminderSentAt.Before(lastWriteTime) {
				emailBody := mail.Get30DaysNoWriteEmail(u.Name)
				err := mail.SendHTMLMail(u.Email, "Yuk, bagikan ide atau postingan baru Anda di Tulis CMS!", emailBody)
				if err == nil {
					u.LastWriteReminderSentAt = &now
					db.Save(&u)
				}
			}
		}
	}
}
