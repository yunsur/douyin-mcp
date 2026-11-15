# 抖音 Web 客户端版本基线

最后复核：2026-11-15

## 当前登录态版本

- `version_code=170400`
- `version_name=17.4.0`
- `update_version_code=170400`
- 直播房间接口：`webcast_sdk_version=2450`

## 复核规则

后续使用 Chrome DevTools 抓取已登录 Web 客户端时，先比较上述版本字段：

1. `version_code`、`version_name`、`update_version_code` 未变化：不重新改 MCP 接口参数。
2. 主版本变化：重新复核已有 MCP 接口的路径、方法、参数、签名和响应格式。
3. 仅 `webcast_sdk_version` 变化：只复核直播相关接口。

版本字段之外的 `support_dash`、`round_trip_time` 属于接口参数，不作为版本变更判断条件；只有出现请求失败或版本变化时才重新抓包确认。
