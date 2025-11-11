# 注册服务
curl -X POST http://172.28.125.175:8080/api/v1/services -H "Content-Type: application/json" -d '{
    "id": "AR2",
    "name": "AR/VR",
    "description": "Augmented Reality Service",
    "computing_requirement": "High",
    "storage_requirement": "Medium",
    "computing_time": "10ms",
    "code_location": "http://example.com/ar1"
}'
# 查看某个ID的服务
curl -X GET http://172.28.125.175:8080/api/v1/services/AR1760332879672

# 查看所有已注册的服务
curl -X GET http://172.28.125.175:8080/api/v1/services

unset http_proxy
unset https_proxy

curl -X POST http://172.28.125.175:8080/validate -H "Content-Type: application/json" -d '{
    "service_id": "AR1760332879672",
    "sample": "test_sample",
    "result": "expected_result"
}'

curl -X GET http://172.22.118.77:8081/metrics

curl -X GET http://172.25.21.118:8082/metrics

curl -X POST http://172.22.118.77:8081/deploy -H "Content-Type: application/json" -d '{
    "service_id": "AR1760332879672",
    "gas": 2
}'


curl -X POST http://172.28.125.175:8084/request_service -H "Content-Type: application/json" -d '{
    "service_id": "AR1760332879672",
    "max_accept_cost": 5,
    "max_accept_delay": 100
}'

curl -X POST http://127.0.0.1:8081/deploy -H "Content-Type: application/json" -d '{
    "service_id": "AR1760332879672",
    "gas": 2
}'

sqlite3 ./db/site1.db "DELETE FROM deployed_services;"

sqlite3 ./db/site1.db "DELETE FROM deployed_services WHERE id = 'AR1760332879672-site-1-1760110562598';"