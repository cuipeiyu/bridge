# bridge

桥接网络

## 如何使用(CentOS7 为例)

- 上传二进制文件与配置文件(.conf)至 ``/usr/local/sbin/`` 下

  ```bash
  ll /usr/local/sbin/
  总用量 2896
  -rw------- 1 root root 2957990 11月 28 17:00 bridge
  -rw------- 1 root root      27 10月 12 22:19 bridge.conf
  ```

- 编辑 .conf 文件

- 上传 ``bridge.service`` 至 ``/usr/lib/systemd/system/`` 下

- 启动服务
  ```bash
  systemctl enable bridge
  systemctl start bridge

  ```

## 定时重启（可选）

```sh
crontab -e
* */2 * * * systemctl restart bridge
```
