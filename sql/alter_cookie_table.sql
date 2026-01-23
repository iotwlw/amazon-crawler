-- 数据库扩展脚本：为 amc_cookie 表添加 Session 绑定字段
-- 用途：支持 Cookie、浏览器指纹、代理 IP 三者绑定
-- 执行时间：2026-01-23

-- 添加浏览器指纹和代理绑定字段
ALTER TABLE `amc_cookie`
ADD COLUMN `browser_profile` VARCHAR(50) DEFAULT NULL COMMENT '绑定的浏览器配置ID（如 chrome-120-win）' AFTER `city`,
ADD COLUMN `proxy_addr` VARCHAR(100) DEFAULT NULL COMMENT '绑定的代理地址（如 192.168.1.1:1080）' AFTER `browser_profile`,
ADD COLUMN `request_count` INT DEFAULT 0 COMMENT '请求次数统计' AFTER `proxy_addr`,
ADD COLUMN `success_count` INT DEFAULT 0 COMMENT '成功次数统计' AFTER `request_count`,
ADD COLUMN `last_request` DATETIME DEFAULT NULL COMMENT '最后请求时间' AFTER `success_count`;

-- 添加索引以提高查询性能
ALTER TABLE `amc_cookie`
ADD INDEX `idx_browser_profile` (`browser_profile`),
ADD INDEX `idx_proxy_addr` (`proxy_addr`),
ADD INDEX `idx_last_request` (`last_request`);

-- 查看修改后的表结构
DESC `amc_cookie`;

-- 查看 Cookie 统计信息（可选）
SELECT
    COUNT(*) as total,
    SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) as active,
    SUM(CASE WHEN status = 0 THEN 1 ELSE 0 END) as invalid,
    SUM(CASE WHEN status = 1 AND host_id IS NULL THEN 1 ELSE 0 END) as unassigned,
    SUM(CASE WHEN status = 1 AND host_id IS NOT NULL THEN 1 ELSE 0 END) as assigned,
    SUM(CASE WHEN browser_profile IS NOT NULL THEN 1 ELSE 0 END) as with_profile,
    SUM(CASE WHEN proxy_addr IS NOT NULL THEN 1 ELSE 0 END) as with_proxy
FROM `amc_cookie`;
