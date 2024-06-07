import requests
from requests.auth import HTTPBasicAuth

def add_sample_data(kibana_url, username, password):
    url = f"{kibana_url}/api/sample_data/logs"
    headers = {
        'kbn-xsrf': 'true',
        'Content-Type': 'application/json'
    }
    
    # Send the POST request
    response = requests.post(url, headers=headers, auth=HTTPBasicAuth(username, password))

    # Check the response status
    if response.status_code == 200:
        print("Sample data added successfully!")
    else:
        print(f"Failed to add sample data. Status code: {response.status_code}")
        print(f"Response: {response.text}")

# Example usage
kibana_url = "http://localhost:5601"
username = "elastic"
password = "default"

add_sample_data(kibana_url, username, password)
