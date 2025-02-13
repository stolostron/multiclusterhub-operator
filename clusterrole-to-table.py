import yaml
import sys
import json

def load_clusterrole(filename):
    with open(filename, "r") as file:
        return yaml.safe_load(file)

def convert_to_table(clusterrole, output_file):
    with open(output_file, "w") as file:
        file.write("| API Groups | Resources | Verbs |\n")
        file.write("|------------|----------|-------|\n")
        
        for rule in clusterrole.get("rules", []):
            api_groups = json.dumps(rule.get("apiGroups", []))
            resources = json.dumps(rule.get("resources", []))
            verbs = json.dumps(rule.get("verbs", []))
            file.write(f"| {api_groups} | {resources} | {verbs} |\n")

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: python script.py <clusterrole.yaml> <output.txt>")
        sys.exit(1)
    
    filename = sys.argv[1]
    output_file = sys.argv[2]
    clusterrole = load_clusterrole(filename)
    convert_to_table(clusterrole, output_file)
    print(f"Table saved to {output_file}")
