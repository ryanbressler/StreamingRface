import sys
import json
import urllib
import urllib2




def nodeFromLabel(label):
	"""generate a hash that represens the json form of a node from a feature lanbe"""
	node_obj={"label":label}
	vs=label.split(":")
	node_obj["chr"]=vs[3]
	node_obj["start"]=vs[4]
	node_obj["end"]=vs[5]
	node_obj["source"]=vs[6]
	return node_obj


def main():
	fo = open(sys.argv[1])
	impCutoff = float(sys.argv[2])
	targetUrl = sys.argv[3]
	
	edges=[]
	for line in fo:
		vs = line.rstrip().split()
		imp = float(vs[3])
		if imp >= impCutoff:
			edges.append({"node1":nodeFromLabel(vs[0]),"node2":nodeFromLabel(vs[1]),"assoc_value1":imp})
	edgesjson = json.dumps(edges)
	data = urllib.urlencode({"results":edgesjson})
	urllib2.urlopen(targetUrl,data)


if __name__ == '__main__':
	main()