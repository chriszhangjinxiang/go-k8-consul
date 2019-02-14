package imp

import
(
	"k8s.io/client-go/1.5/kubernetes"
	"fmt"
	"flag"
	"k8s.io/client-go/1.5/tools/clientcmd"
	metav1 "k8s.io/client-go/1.5/pkg/api"
	consulapi "github.com/hashicorp/consul/api"
	"reflect"
	"strconv"
	"k8s.io/client-go/1.5/pkg/api/v1"
)


func GetService(k8path string, consuladdress string){
	var kubeconfig *string
	fmt.Print("to get service from kubernets cluster!")
	//kubeconfig = flag.String("kubeconfig","/etc/kubeconf/config-linux","(optional)absolute path to the kubecofnig file")
	if k8path == "" {
		k8path = "./config"
	}
	kubeconfig = flag.String("kubeconfig",k8path,"(optional)absolute path to the kubecofnig file")
	config, err := clientcmd.BuildConfigFromFlags("",*kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	consulconfig := consulapi.Config{
		Address:consuladdress}

	//config := consulapi.Config{
	//	Address:"172.31.150.195:8500"}

	consulclient, errconsul := consulapi.NewClient(&consulconfig)
	if errconsul != nil {
		fmt.Println("consul error!" + errconsul.Error())
	}

	//use channel to start service & pod monitor, if monitor restart, the channel will receive message, then can do operation accrodingly
	flagCH := make(chan string)
	go serviceWatch(*clientset,flagCH,*consulclient)
	go podWatch(*clientset,*consulclient,flagCH)
	for {
		flag, ok := <- flagCH
		if ok {
			print(flag + ": it is flag value")
			if flag == "00" {
				go serviceWatch(*clientset,flagCH,*consulclient)
			} else if flag == "01" {
				go podWatch(*clientset,*consulclient,flagCH)
			}
		} else {
			print("channel closed! the service & pod monitor will be in unstalbe status!")
		}
	}
	var str string
	fmt.Scan(&str)

}

func podWatch(clientset kubernetes.Clientset,consulclient consulapi.Client,flagch chan string) {

	fmt.Print("start monitor pod")
	podwatch, _ := clientset.Core().Pods("default").Watch(metav1.ListOptions{})
		outflag1:
		for {
			select {
			case e, ok := <-podwatch.ResultChan():
				if ok && e.Type == "ADDED" {
					//fmt.Println(e.Type, e.Object)
					val := reflect.ValueOf(e.Object).Elem()
					pod, ok := val.Interface().(v1.Pod)
					if ok {

						//use labels to find out coresponding k8 service
						serviceList, err := clientset.Services("default").List(metav1.ListOptions{})
						if err != nil {
							panic(err.Error())
						}
						var cor_service string
						var nodePort int
					outflag:
						for key, value := range pod.Labels {
							for i := 0; i < len(serviceList.Items); i++ {
								if va, ex := serviceList.Items[i].Spec.Selector[key]; ex {
									if value == va {
										//this is the service we find out, pod lable key equals service label key and value also equals
										cor_service = serviceList.Items[i].Name
										nodePort = int(serviceList.Items[i].Spec.Ports[0].NodePort)
										break outflag
									}
								}

							}
						}

						//if service exists, then check if registred in consul
						if cor_service != "" {
							var exist_flag bool
							services, _ := consulclient.Agent().Services()
							for _,v := range services {
								if v.Service == cor_service {
									exist_flag = true
								}
							}
							if exist_flag {
								// the service exist already, do nothing
							} else {
								//the service does not exist, and the service exist in k8, so need to do the registration
								nodes, err := clientset.Nodes().List(metav1.ListOptions{})
								if err != nil {
									panic(err.Error())
								}
								for i := 0; i < len(nodes.Items); i++ {
									doConsulRegistration(consulclient, cor_service, nodePort, nodes.Items[i].Name)
								}
							}
						}
					}
				}else {
					fmt.Print("channel closed! this go routin will quit!\n")
					flagch <- "01"
					fmt.Print("in child go routin set chan value!\n")
					break outflag1
				}
			}
		}
}

func serviceWatch(clientset kubernetes.Clientset,flagch chan string,consulclient consulapi.Client) {
	fmt.Print("start monitor service!")
	servicewatch, _ := clientset.Services("default").Watch(metav1.ListOptions{})
	nodes,_ := clientset.Nodes().List(metav1.ListOptions{})
	services,_ := consulclient.Agent().Services()
	var m1 map[string]string = map[string]string{}
	for _, v := range services {
		hel,_,err := consulclient.Health().Checks(v.Service,nil)
		if err == nil && hel.AggregatedStatus() == "passing" {
			str := v.Address + ":"
			str += strconv.Itoa(v.Port)
			m1[str] = v.Service
		}
	}
	outflag:
	for {
		select {
		case e, ok := <- servicewatch.ResultChan():
			if ok {
				val := reflect.ValueOf(e.Object).Elem()
				serviceName :=val.FieldByName("Name").String()
				port :=val.FieldByName("Spec").FieldByName("Ports")
				sp1, ok := port.Interface().([]v1.ServicePort)
				if ok {
					for i:=0; i<len(sp1); i++ {

						for j:=0; j<len(nodes.Items); j++ {
							nodePort := sp1[i].NodePort
							ipAddress := nodes.Items[j].GetName()
							str :=  ipAddress +":" + strconv.Itoa(int(nodePort))
							// to check if the service exist in the current consul health service
							if _, ok1 := m1[str]; ok1 {
								if e.Type == "DELETED" {
									fmt.Println("to delete services!" + serviceName)
								}
							} else {
								// if not exist, register to consul
								if e.Type == "ADDED" {
									doConsulRegistration(consulclient, serviceName, int(nodePort), ipAddress)
								}
							}
						}
					}
				} else {
					fmt.Println("it is not serviceport")
				}
			} else {
				fmt.Print("channel closed! this go routin will quit!\n")
				flagch <- "00"
				fmt.Print("in child go routin set chan value!\n")
				break outflag
			}
		}
	}
}


//to register service in consul
func doConsulRegistration(client consulapi.Client, serviceName string, port int, IPAddress string ) {

	checkPort := port
	registration := new(consulapi.AgentServiceRegistration)
	registration.ID = serviceName + "_" + IPAddress + "_" + strconv.Itoa(port)
	registration.Name = serviceName
	registration.Port = port
	registration.Tags = []string{"ENV=TEST","VERSION=V1.0"}
	registration.Address = IPAddress
	registration.Check = &consulapi.AgentServiceCheck{
		//HTTP:                           fmt.Sprintf("--cache=off http://%s:%d%s", registration.Address, checkPort, "/"),
		TCP:							fmt.Sprintf("%s:%d",registration.Address, checkPort),
		Timeout:                        "3s",
		Interval:                       "5s",
		DeregisterCriticalServiceAfter: "30s", //check失败后30秒删除本服务
	}


	err := client.Agent().ServiceRegister(registration)

	if err != nil {
		fmt.Println("register server error : ", err)
	} else {
		fmt.Println("service registration sucessful, service name is:" + serviceName)
	}
}

