-- amc_category 表结构变更
-- 增加任务状态、创建时间、更新时间字段

-- 任务状态: 0=待执行, 1=已执行, 2=失败
ALTER TABLE `amc_category`
ADD COLUMN `task_status` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '任务状态: 0=待执行, 1=已执行, 2=失败',
ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间';

-- 为任务状态添加索引，便于查询待执行任务
ALTER TABLE `amc_category` ADD INDEX `idx_task_status` (`task_status`);

-- 将现有数据的状态设为已执行（可选，根据需要执行）
-- UPDATE `amc_category` SET `task_status` = 1 WHERE `task_status` = 0;
