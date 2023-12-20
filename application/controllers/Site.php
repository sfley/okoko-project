<?php

Class Site extends Ci_Controller {
	
	function index(){
		$this->load->view('menu');
	    $this->load->model('Boiler');
		$data['info'] = $this->Boiler->get_all();
		$this->load->view('info',$data);
		$this->load->view('footer',$data);
					}
					
					
	function modulation(){
	    $this->load->model('modulation');
		 $this->load->model('Boiler');
		$data['modulation'] = $this->modulation->get();
		$data['info'] = $this->Boiler->get_all();
		$this->load->view('menu');
		$this->load->view('modulation',$data);
		$this->load->view('footer',$data);
					}	

			
					
					
	function temp(){
	    $this->load->model('Temp');
		 $this->load->model('Boiler');
		$data['tempHK1'] = $this->Temp->get_tempHK1();
		$data['tempHK2'] = $this->Temp->get_tempHK2();
		$data['tempWW1'] = $this->Temp->get_tempWW1();
		$data['tempEXT'] = $this->Temp->get_tempEXT();
		$data['info'] = $this->Boiler->get_all();
		$this->load->view('menu');
		$this->load->view('temp',$data);
		$this->load->view('footer',$data);
					}
					
					
						function test(){
	   
		 $this->load->model('Temp');
		 $this->load->model('Boiler');
		$data['tempHK1'] = $this->Temp->get_tempHK1();
		$data['tempHK2'] = $this->Temp->get_tempHK2();
		$data['tempWW1'] = $this->Temp->get_tempWW1();
		$data['tempEXT'] = $this->Temp->get_tempEXT();
		$data['info'] = $this->Boiler->get_all();
		$this->load->view('menu');
		$this->load->view('test',$data);
		$this->load->view('footer',$data);
	
					}
	
	
								
function conso(){
	    $this->load->model('Conso');
		$this->load->model('Boiler');
		$data['info'] = $this->Boiler->get_all();
		$data['conso_annuelle'] = $this->Conso->get_conso(2023);
		$this->load->view('menu');
		$this->load->view('conso',$data);		
		$this->load->view('footer',$data);
					}	



	function stock(){
	    $this->load->model('Boiler');
		$this->load->model('Conso');
		$data['conso_now'] = $this->Conso->get_conso_now();		
		$data['info'] = $this->Boiler->get_all();
		$this->load->view('menu');
		$this->load->view('stock',$data);
		$this->load->view('footer',$data);
					}					
					
					
					
					
					
					
					
					
								}