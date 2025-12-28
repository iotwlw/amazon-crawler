-- 域名关键词插入脚本
-- 中文词统一为 "域名"，priority 默认为 0

USE taotie;

-- 清空现有的域名相关数据（可选）
-- DELETE FROM amc_category WHERE zh_key = '域名';

-- 插入域名关键词
INSERT INTO `amc_category` (`zh_key`, `en_key`, `priority`) VALUES
('域名', 'exurbiz', 0),
('域名', 'hllbll', 0),
('域名', 'chamixx', 0),
('域名', 'xximuim', 0),
('域名', 'peacolate', 0),
('域名', 'aryiten', 0),
('域名', 'navnihaal', 0),
('域名', 'maozhren', 0),
('域名', 'aopigavi', 0),
('域名', 'koroao', 0),
('域名', 'motofoal', 0),
('域名', 'kejsted', 0),
('域名', 'syowada', 0),
('域名', 'muhize', 0),
('域名', 'byczone', 0),
('域名', 'munetoshi', 0),
('域名', 'fasworx', 0),
('域名', 'strongthium', 0),
('域名', 'syunsxoon', 0),
('域名', 'ghamade', 0),
('域名', 'osompar', 0),
('域名', 'aladiche', 0),
('域名', 'lklkkc', 0),
('域名', 'coucoland', 0),
('域名', 'laffoonparts', 0),
('域名', 'hihiav', 0),
('域名', 'kozhom', 0),
('域名', 'aofure', 0),
('域名', 'camotokiit', 0),
('域名', 'geeoollah', 0),
('域名', 'magtsmei', 0),
('域名', 'adllya', 0),
('域名', 'uiihunt', 0),
('域名', 'sinvimes', 0),
('域名', 'ciloyu', 0),
('域名', 'fzjdsd', 0),
('域名', 'marsrut', 0),
('域名', 'kpalag', 0),
('域名', 'jayobgo', 0),
('域名', 'calofulston', 0),
('域名', 'himarklif', 0),
('域名', 'ielek', 0),
('域名', 'pifoog', 0),
('域名', 'boardfeb', 0),
('域名', 'taurusy', 0),
('域名', 'mixumon', 0),
('域名', 'deabolar', 0),
('域名', 'lileipower', 0),
('域名', 'loyaforba', 0),
('域名', 'oyrel', 0),
('域名', 'viatabuna', 0),
('域名', 'auloor', 0),
('域名', 'gemwi', 0),
('域名', 'supbri', 0),
('域名', 'viodaim', 0),
('域名', 'wanjiaone', 0),
('域名', 'voncerus', 0),
('域名', 'szzyxd', 0),
('域名', 'ledholyt', 0),
('域名', 'eaasty', 0),
('域名', 'goyha', 0),
('域名', 'voulosimi', 0),
('域名', 'ibosins', 0),
('域名', 'penxua', 0),
('域名', 'halatool', 0),
('域名', 'zryxal', 0),
('域名', 'hgwalp', 0),
('域名', 'supveco', 0),
('域名', 'zmozn', 0),
('域名', 'konictom', 0),
('域名', 'vanthylit', 0),
('域名', 'aiyuanyin', 0),
('域名', 'yuanyinai', 0);

-- 验证插入结果
SELECT COUNT(*) AS '插入的关键词数量' FROM amc_category WHERE zh_key = '域名';

-- 查看插入的关键词
SELECT * FROM amc_category WHERE zh_key = '域名' ORDER BY id;
