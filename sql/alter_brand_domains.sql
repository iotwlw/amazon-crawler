-- 品牌巡查功能 - 数据库变更脚本
-- 为 available_brand_domains 表添加巡查状态字段

-- 添加巡查状态字段
ALTER TABLE `available_brand_domains`
ADD COLUMN `patrol_status` TINYINT(1) NOT NULL DEFAULT 0
    COMMENT '巡查状态: 0=待巡查, 1=巡查中, 2=已完成, 3=失败, 4=无结果',
ADD COLUMN `patrol_app_id` TINYINT(1) DEFAULT NULL
    COMMENT '处理该记录的程序实例ID',
ADD COLUMN `patrol_error` VARCHAR(255) DEFAULT NULL
    COMMENT '巡查错误信息';

-- 添加索引
ALTER TABLE `available_brand_domains`
ADD INDEX `idx_patrol_status` (`patrol_status`);

-- 验证变更
-- SELECT COLUMN_NAME, COLUMN_TYPE, COLUMN_COMMENT
-- FROM INFORMATION_SCHEMA.COLUMNS
-- WHERE TABLE_NAME = 'available_brand_domains' AND COLUMN_NAME LIKE 'patrol%';
