import requests
import json
import random
import time

def generate_random_ip():
    """Generates a random, valid IPv4 address string."""
    # Ensure the first octet is not 0, 10 (private), 127 (loopback),
    # 169.254 (link-local), or 224-255 (multicast/reserved).
    # This gives a range of generally "public" and usable IPs for testing.
    first_octet = random.randint(1, 223)
    while first_octet == 10 or first_octet == 127 or first_octet == 169: # 169.254.x.x is link-local
        first_octet = random.randint(1, 223)

    return f"{first_octet}.{random.randint(0, 255)}.{random.randint(0, 255)}.{random.randint(0, 255)}"

def send_ip_to_api(ip_address, api_url="http://dreadco:8080/ip4"):
    """
    Sends a single IP address to the specified API endpoint.
    The Go API expects a JSON body like: {"value": "xxx.xxx.xxx.xxx"}
    """
    payload = {"value": ip_address}
    headers = {"Content-Type": "application/json"}
    try:
        # Setting a timeout for the request (e.g., 5 seconds)
        response = requests.post(api_url, data=json.dumps(payload), headers=headers, timeout=5)
        # Check if the request was successful
        if response.status_code == 200:
            print(f"Successfully sent IP: {ip_address} - Response: {response.json()}")
            return True
        else:
            print(f"Failed to send IP: {ip_address} - Status: {response.status_code} - Response: {response.text}")
            return False
    except requests.exceptions.Timeout:
        print(f"Timeout error sending IP: {ip_address}")
        return False
    except requests.exceptions.ConnectionError:
        print(f"Connection error sending IP: {ip_address} - Is the server running at {api_url}?")
        return False
    except requests.exceptions.RequestException as e:
        print(f"General error sending IP: {ip_address} - Exception: {e}")
        return False

def main():
    num_ips_to_generate = 1300  # Number of IPs to generate and send
    target_api_url = "http://dreadco:8080/ip4" # Your Go application's /ip4 endpoint
    delay_between_requests = 0.05  # Seconds to wait between requests (adjust as needed to not overwhelm the server)

    print(f"Attempting to generate and send {num_ips_to_generate} unique IP addresses to {target_api_url}")

    generated_ips = set()
    successful_sends = 0
    failed_sends = 0

    # Generate unique IPs first to avoid sending duplicates
    # and to ensure we attempt to send the target number of *unique* IPs.
    print("Generating unique IP addresses...")
    while len(generated_ips) < num_ips_to_generate:
        ip = generate_random_ip()
        generated_ips.add(ip)
        if len(generated_ips) % 100 == 0 and len(generated_ips) > 0:
            print(f"Generated {len(generated_ips)} unique IPs so far...")
    
    print(f"\nStarting to send {len(generated_ips)} unique IPs...\n")

    ips_to_send = list(generated_ips) # Convert set to list for ordered iteration

    for i, ip in enumerate(ips_to_send):
        if send_ip_to_api(ip, target_api_url):
            successful_sends += 1
        else:
            failed_sends += 1
        
        # Optional: print progress during sending
        if (i + 1) % 50 == 0:
            print(f"Sent {i + 1}/{len(ips_to_send)} IPs...")

        #if delay_between_requests > 0:
        #    time.sleep(delay_between_requests)

    print("\n--- Test Summary ---")
    print(f"Total unique IPs targeted: {num_ips_to_generate}")
    print(f"Total unique IPs generated and attempted to send: {len(ips_to_send)}")
    print(f"Successfully sent: {successful_sends}")
    print(f"Failed to send: {failed_sends}")

if __name__ == "__main__":
    main()

