function dymcp_stop
    launchctl stop dymcp
end

function dymcp_start
    launchctl start dymcp
end

function dymcp_status
    set service_name "dymcp"

    # 获取服务状态
    set pid_status (launchctl list | grep $service_name | awk '{print $1}')

    if test "$pid_status" != "-"
        echo "✓ $service_name 正在运行 (PID: $pid_status)"
        read -P "是否停止服务? (yes/其他): " answer
        if test "$answer" = "yes"
            dymcp_stop
            echo "✓ 服务已停止"
        else
            echo "取消操作"
        end
    else
        echo "✗ $service_name 未运行"
        read -P "是否启动服务? (yes/其他): " answer
        if test "$answer" = "yes"
            dymcp_start
            sleep 1
            set pid_status (launchctl list | grep $service_name | awk '{print $1}')
            if test "$pid_status" != "-"
                echo "✓ 服务启动成功 (PID: $pid_status)"
            else
                echo "✗ 服务启动失败，检查日志: /tmp/dymcp.err"
                return 1
            end
        else
            echo "取消操作"
            return 1
        end
    end
end
