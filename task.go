package main

import (
	"database/sql"
	"sync"
	"time"

	log "github.com/tengfei-xy/go-log"
)

// CrawlTask 表示一个爬取任务
type CrawlTask struct {
	ID      int64  // 数据库记录 ID
	Keyword string // 品牌名/关键词
}

// 任务通知 channel，用于唤醒 Worker
var taskNotify = make(chan struct{}, 1)

// TaskWorker 任务消费者
type TaskWorker struct {
	wg      sync.WaitGroup
	stopCh  chan struct{}
	running bool
}

var taskWorker *TaskWorker

// InitTaskWorker 初始化任务消费者
func InitTaskWorker() {
	taskWorker = &TaskWorker{
		stopCh: make(chan struct{}),
	}
}

// Start 启动任务消费者
func (tw *TaskWorker) Start() {
	tw.wg.Add(1)
	tw.running = true
	go func() {
		defer tw.wg.Done()
		log.Info("任务消费者已启动，等待任务...")

		// 定时检查间隔
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-tw.stopCh:
				log.Info("任务消费者收到停止信号")
				return
			case <-taskNotify:
				// 收到新任务通知，立即处理
				tw.processPendingTasks()
			case <-ticker.C:
				// 定时检查待执行任务
				tw.processPendingTasks()
			}
		}
	}()
}

// Stop 停止任务消费者
func (tw *TaskWorker) Stop() {
	if tw.running {
		close(tw.stopCh)
		tw.wg.Wait()
		tw.running = false
		log.Info("任务消费者已停止")
	}
}

// processPendingTasks 处理待执行的任务
func (tw *TaskWorker) processPendingTasks() {
	for {
		task, err := tw.fetchNextTask()
		if err != nil {
			if err == sql.ErrNoRows {
				// 没有待执行任务
				return
			}
			log.Errorf("获取任务失败: %v", err)
			return
		}

		log.Infof("========================================")
		log.Infof("开始执行任务 ID:%d 关键词:%s", task.ID, task.Keyword)
		log.Infof("========================================")

		// 执行爬取任务
		success := ExecuteCrawlWithStatus(task)

		if success {
			tw.updateTaskStatus(task.ID, TASK_STATUS_COMPLETED)
			log.Infof("任务执行成功 ID:%d 关键词:%s", task.ID, task.Keyword)
		} else {
			tw.updateTaskStatus(task.ID, TASK_STATUS_FAILED)
			log.Errorf("任务执行失败 ID:%d 关键词:%s", task.ID, task.Keyword)
		}
	}
}

// fetchNextTask 从数据库获取下一个待执行任务
func (tw *TaskWorker) fetchNextTask() (CrawlTask, error) {
	var task CrawlTask

	// 查询一条待执行任务
	err := app.db.QueryRow(
		"SELECT id, en_key FROM amc_category WHERE task_status = ? ORDER BY id ASC LIMIT 1",
		TASK_STATUS_PENDING,
	).Scan(&task.ID, &task.Keyword)

	return task, err
}

// updateTaskStatus 更新任务状态
func (tw *TaskWorker) updateTaskStatus(id int64, status int) {
	_, err := app.db.Exec(
		"UPDATE amc_category SET task_status = ? WHERE id = ?",
		status, id,
	)
	if err != nil {
		log.Errorf("更新任务状态失败 ID:%d 状态:%d 错误:%v", id, status, err)
	}
}
