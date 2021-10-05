@run = true

message = ARGV[0] || "HELLO"

Signal.trap("INT") do
  puts "Terminating..."
  sleep 7
  puts "done"
  @run = false
  exit 0
end

while @run
  puts message
  sleep 1
end
