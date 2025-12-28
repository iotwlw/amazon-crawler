
-- MySQL dump 10.13  Distrib 5.7.35, for Linux (x86_64)
--
-- Host: localhost    Database: taotie
-- ------------------------------------------------------
-- Server version	5.7.35-log

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Current Database: `taotie`
--

CREATE DATABASE /*!32312 IF NOT EXISTS*/ `taotie` /*!40100 DEFAULT CHARACTER SET utf8mb4 */;

USE `taotie`;

--
-- Temporary table structure for view `产品检查表`
--

DROP TABLE IF EXISTS `产品检查表`;
/*!50001 DROP VIEW IF EXISTS `产品检查表`*/;
SET @saved_cs_client     = @@character_set_client;
SET character_set_client = utf8;
/*!50001 CREATE VIEW `产品检查表` AS SELECT 
 1 AS `状态`,
 1 AS `链接数量`*/;
SET character_set_client = @saved_cs_client;

--
-- Temporary table structure for view `占用空间表`
--

DROP TABLE IF EXISTS `占用空间表`;
/*!50001 DROP VIEW IF EXISTS `占用空间表`*/;
SET @saved_cs_client     = @@character_set_client;
SET character_set_client = utf8;
/*!50001 CREATE VIEW `占用空间表` AS SELECT 
 1 AS `Database`,
 1 AS `Size (MB)`*/;
SET character_set_client = @saved_cs_client;

--
-- Temporary table structure for view `商家信息表`
--

DROP TABLE IF EXISTS `商家信息表`;
/*!50001 DROP VIEW IF EXISTS `商家信息表`*/;
SET @saved_cs_client     = @@character_set_client;
SET character_set_client = utf8;
/*!50001 CREATE VIEW `商家信息表` AS SELECT 
 1 AS `商家ID`,
 1 AS `名称`,
 1 AS `地址`,
 1 AS `税号`,
 1 AS `税号标识`,
 1 AS `信息标识`*/;
SET character_set_client = @saved_cs_client;

--
-- Temporary table structure for view `商家trn表`
--

DROP TABLE IF EXISTS `商家trn表`;
/*!50001 DROP VIEW IF EXISTS `商家trn表`*/;
SET @saved_cs_client     = @@character_set_client;
SET character_set_client = utf8;
/*!50001 CREATE VIEW `商家trn表` AS SELECT 
 1 AS `数量`,
 1 AS `count(*)`*/;
SET character_set_client = @saved_cs_client;

--
-- Temporary table structure for view `搜索统计表`
--

DROP TABLE IF EXISTS `搜索统计表`;
/*!50001 DROP VIEW IF EXISTS `搜索统计表`*/;
SET @saved_cs_client     = @@character_set_client;
SET character_set_client = utf8;
/*!50001 CREATE VIEW `搜索统计表` AS SELECT 
 1 AS `中文关键词`,
 1 AS `搜索次数`,
 1 AS `产品数`*/;
SET character_set_client = @saved_cs_client;

--
-- Temporary table structure for view `程序状态表`
--

DROP TABLE IF EXISTS `程序状态表`;
/*!50001 DROP VIEW IF EXISTS `程序状态表`*/;
SET @saved_cs_client     = @@character_set_client;
SET character_set_client = utf8;
/*!50001 CREATE VIEW `程序状态表` AS SELECT 
 1 AS `app_id`,
 1 AS `状态`,
 1 AS `更新时间`*/;
SET character_set_client = @saved_cs_client;

--
-- Temporary table structure for view `类别总数表`
--

DROP TABLE IF EXISTS `类别总数表`;
/*!50001 DROP VIEW IF EXISTS `类别总数表`*/;
SET @saved_cs_client     = @@character_set_client;
SET character_set_client = utf8;
/*!50001 CREATE VIEW `类别总数表` AS SELECT 
 1 AS `类别总数`*/;
SET character_set_client = @saved_cs_client;

--
-- Table structure for table `application`
--

DROP TABLE IF EXISTS `amc_application`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `amc_application` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `app_id` int(11) NOT NULL,
  `status` tinyint(1) NOT NULL DEFAULT '0',
  `update` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=177 DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `category`
--

DROP TABLE IF EXISTS `amc_category`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `amc_category` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `zh_key` varchar(30) NOT NULL,
  `en_key` varchar(50) NOT NULL,
  `priority` int(11) DEFAULT '0',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=1016 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `cookie`
--

DROP TABLE IF EXISTS `amc_cookie`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `amc_cookie` (
  `host_id` tinyint(1) NOT NULL,
  `cookie` text,
  PRIMARY KEY (`host_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `product`
--

DROP TABLE IF EXISTS `amc_product`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `amc_product` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `url` varchar(200) NOT NULL,
  `param` varchar(1000) NOT NULL,
  `title` varchar(500) DEFAULT NULL,
  `asin` varchar(50) DEFAULT NULL,
  `keyword` varchar(100) DEFAULT NULL,
  `bought_count` varchar(50) DEFAULT NULL,
  `price` varchar(50) DEFAULT NULL,
  `rating` varchar(10) DEFAULT NULL,
  `review_count` varchar(50) DEFAULT NULL,
  `seller_id` varchar(25) DEFAULT NULL,
  `brand_name` varchar(100) DEFAULT NULL,
  `brand_store_url` varchar(500) DEFAULT NULL,
  `status` tinyint(1) DEFAULT '0',
  `app` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `url` (`url`)
) ENGINE=InnoDB AUTO_INCREMENT=462720 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `search_statistics`
--

DROP TABLE IF EXISTS `amc_search_statistics`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `amc_search_statistics` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `category_id` int(11) NOT NULL,
  `start` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `end` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `status` tinyint(1) DEFAULT '0',
  `app` tinyint(1) NOT NULL,
  `valid` int(11) DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `category_id` (`category_id`),
  CONSTRAINT `amc_search_statistics_ibfk_1` FOREIGN KEY (`category_id`) REFERENCES `amc_category` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2352 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `seller`
--

DROP TABLE IF EXISTS `amc_seller`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `amc_seller` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `seller_id` varchar(25) NOT NULL,
  `name` varchar(200) DEFAULT NULL,
  `seller_name` varchar(200) DEFAULT NULL,
  `keyword` varchar(100) DEFAULT NULL,
  `address` varchar(200) DEFAULT NULL,
  `trn` varchar(28) DEFAULT NULL,
  `trn_status` tinyint(1) NOT NULL DEFAULT '0',
  `all_status` tinyint(1) DEFAULT '0',
  `app_id` tinyint(1) DEFAULT '0',
  `company_id` char(16) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `seller_id_UNIQUE` (`seller_id`)
) ENGINE=InnoDB AUTO_INCREMENT=115634 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Current Database: `taotie`
--

USE `taotie`;

--
-- Final view structure for view `产品检查表`
--

/*!50001 DROP VIEW IF EXISTS `产品检查表`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8mb4 */;
/*!50001 SET character_set_results     = utf8mb4 */;
/*!50001 SET collation_connection      = utf8mb4_general_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`taotie`@`%` SQL SECURITY DEFINER */
/*!50001 VIEW `产品检查表` AS select (case when (`amc_product`.`status` = 0) then '未搜索' when (`amc_product`.`status` = 1) then '准备检查' when (`amc_product`.`status` = 2) then '检查结束' when (`amc_product`.`status` = 3) then '其他错误' when (`amc_product`.`status` = 4) then '没有商家' else `amc_product`.`status` end) AS `状态`,count(0) AS `链接数量` from `amc_product` group by `amc_product`.`status` */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;

--
-- Final view structure for view `占用空间表`
--

/*!50001 DROP VIEW IF EXISTS `占用空间表`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8mb4 */;
/*!50001 SET character_set_results     = utf8mb4 */;
/*!50001 SET collation_connection      = utf8mb4_general_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`taotie`@`%` SQL SECURITY DEFINER */
/*!50001 VIEW `占用空间表` AS select `information_schema`.`tables`.`TABLE_SCHEMA` AS `Database`,((sum((`information_schema`.`tables`.`DATA_LENGTH` + `information_schema`.`tables`.`INDEX_LENGTH`)) / 1024) / 1024) AS `Size (MB)` from `information_schema`.`tables` group by `information_schema`.`tables`.`TABLE_SCHEMA` */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;

--
-- Final view structure for view `商家信息表`
--

/*!50001 DROP VIEW IF EXISTS `商家信息表`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8mb4 */;
/*!50001 SET character_set_results     = utf8mb4 */;
/*!50001 SET collation_connection      = utf8mb4_general_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`taotie`@`%` SQL SECURITY DEFINER */
/*!50001 VIEW `商家信息表` AS select `amc_seller`.`seller_id` AS `商家ID`,`amc_seller`.`name` AS `名称`,`amc_seller`.`address` AS `地址`,`amc_seller`.`trn` AS `税号`,(case `amc_seller`.`trn_status` when 0 then 'TRN未查找' when 1 then '中国TRN' when 2 then '空TRN' when 3 then '其他TRN' when 4 then '异常TRN' end) AS `税号标识`,(case `amc_seller`.`all_status` when 0 then '未查找' when 1 then '信息完整' when 2 then '没有名称' when 3 then '没有地址' when 4 then '没有TRN' end) AS `信息标识` from `amc_seller` */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;

--
-- Final view structure for view `商家trn表`
--

/*!50001 DROP VIEW IF EXISTS `商家trn表`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8mb4 */;
/*!50001 SET character_set_results     = utf8mb4 */;
/*!50001 SET collation_connection      = utf8mb4_general_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`taotie`@`%` SQL SECURITY DEFINER */
/*!50001 VIEW `商家trn表` AS select (case `amc_seller`.`trn_status` when 0 then 'TRN未查找' when 1 then '中国TRN' when 2 then '空TRN' when 3 then '其他TRN' when 4 then '异常TRN' end) AS `数量`,count(0) AS `count(*)` from `amc_seller` group by `amc_seller`.`trn_status` */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;

--
-- Final view structure for view `搜索统计表`
--

/*!50001 DROP VIEW IF EXISTS `搜索统计表`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8mb4 */;
/*!50001 SET character_set_results     = utf8mb4 */;
/*!50001 SET collation_connection      = utf8mb4_general_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`taotie`@`%` SQL SECURITY DEFINER */
/*!50001 VIEW `搜索统计表` AS select `k`.`zh_key` AS `中文关键词`,count(0) AS `搜索次数`,sum(`s`.`valid`) AS `产品数` from (`taotie`.`amc_search_statistics` `s` join (select `taotie`.`amc_category`.`id` AS `id`,`taotie`.`amc_category`.`zh_key` AS `zh_key` from `taotie`.`amc_category`) `k` on((`s`.`category_id` = `k`.`id`))) group by `s`.`category_id` */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;

--
-- Final view structure for view `程序状态表`
--

/*!50001 DROP VIEW IF EXISTS `程序状态表`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8mb4 */;
/*!50001 SET character_set_results     = utf8mb4 */;
/*!50001 SET collation_connection      = utf8mb4_general_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`taotie`@`%` SQL SECURITY DEFINER */
/*!50001 VIEW `程序状态表` AS select `amc_application`.`app_id` AS `app_id`,(case when (`amc_application`.`status` = 0) then '启动中' when (`amc_application`.`status` = 1) then '结束' when (`amc_application`.`status` = 2) then '1.搜索页面中' when (`amc_application`.`status` = 3) then '2.查找商家中' when (`amc_application`.`status` = 4) then '3.确定TRN中' end) AS `状态`,`amc_application`.`update` AS `更新时间` from `amc_application` order by `amc_application`.`update` desc */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;

--
-- Final view structure for view `类别总数表`
--

/*!50001 DROP VIEW IF EXISTS `类别总数表`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8mb4 */;
/*!50001 SET character_set_results     = utf8mb4 */;
/*!50001 SET collation_connection      = utf8mb4_general_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`taotie`@`%` SQL SECURITY DEFINER */
/*!50001 VIEW `类别总数表` AS select count(0) AS `类别总数` from `amc_category` */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2024-02-06 12:22:02