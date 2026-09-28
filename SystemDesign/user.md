## 注册
输入：
- 用户名（英文）
- 密码
- 邀请码

返回：


处理逻辑：
1. 验证redis有无对应邀请码，无则拒绝，有则获取对应邀请人id
2. 密码加密（对称加密）
3. insert用户信息至数据表dc_user中

## 登录
输入：
- 用户名
- 密码

处理逻辑：
1. 验证用户信息
2. 生成access_token与refresh_token

## logout
删除redis中的refresh_token