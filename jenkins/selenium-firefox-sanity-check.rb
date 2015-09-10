require "selenium-webdriver"

test_uri = "file://" + File.expand_path(File.dirname(__FILE__) + "/selenium-firefox-sanity-check.html")

# Initialize firefox driver
begin
  driver = Selenium::WebDriver.for :firefox
rescue
  STDERR.puts "Selenium::WebDriver could not initialize firefox driver"
  exit 1
end

# Navigate to test file
begin
  driver.navigate.to test_uri
rescue
  STDERR.puts "Selenium::WebDriver firefox driver could not navigate to test page " + test_uri
  exit 2
end

# Verify that Selenium can find an element in test file
begin
  element = driver.find_element(:id, 'test')
rescue
  STDERR.puts "Selenium::WebDriver firefox driver could not find element in test page"
  exit 3
end

driver.quit
exit 0
