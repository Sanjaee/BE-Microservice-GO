-- Buat DB tambahan untuk microservices
-- userdb sudah dibuat sebagai default database di docker-compose.yml

-- Buat database productdb jika belum ada
SELECT 'CREATE DATABASE productdb'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'productdb')\gexec

-- Buat database paymentdb jika belum ada  
SELECT 'CREATE DATABASE paymentdb'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'paymentdb')\gexec
