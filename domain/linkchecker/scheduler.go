package linkchecker

import (
	"time"

	"github.com/dankedev/tulis-go/domain/post"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// StartLinkCheckerScheduler periodically scans all workspaces for broken links.
// interval = how often to run; threshold = broken-link count that triggers admin email.
func StartLinkCheckerScheduler(db *gorm.DB, interval time.Duration, threshold int) {
	if db == nil {
		logrus.Warn("[LINKCHECKER] Database connection is nil, scheduler not started")
		return
	}
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	go func() {
		logrus.Info("[LINKCHECKER] Initializing broken-link checker scheduler...")
		// Delay first run 30s after startup to let the app settle.
		time.Sleep(30 * time.Second)
		for {
			runAllWorkspaces(db, threshold)
			time.Sleep(interval)
		}
	}()
}

func runAllWorkspaces(db *gorm.DB, threshold int) {
	logrus.Info("[LINKCHECKER] Scanning workspaces for broken links...")
	var workspaces []workspace.Workspace
	if err := db.Find(&workspaces).Error; err != nil {
		logrus.Errorf("[LINKCHECKER] failed to list workspaces: %v", err)
		return
	}

	repo := NewRepository(db)
	postRepo := post.NewPostRepository(db)
	svc := NewService(repo, postRepo, db, threshold)

	for _, ws := range workspaces {
		checked, broken, err := svc.CheckWorkspace(nil, ws.ID)
		if err != nil {
			logrus.Errorf("[LINKCHECKER] workspace %s scan error: %v", ws.ID, err)
			continue
		}
		logrus.Infof("[LINKCHECKER] workspace %s: checked %d posts, found %d broken links", ws.ID, checked, broken)
	}
}
