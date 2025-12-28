-- 更新product表，添加brand_store_url字段

USE `amazon`;

ALTER TABLE `product` 
ADD COLUMN `brand_store_url` varchar(500) DEFAULT NULL COMMENT '品牌旗舰店链接';
