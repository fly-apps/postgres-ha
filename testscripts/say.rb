@run = true

message = ARGV[0] || "HELLO"

Signal.trap("INT") do
  puts "Terminating..."
  @run = false
end

while @run
  puts message
  sleep 1
end
