<?php
Class boiler extends CI_Model {
	
function get_all() {
mb_internal_encoding("UTF-8");

//$url_chaudiere = "http://X.X.X.X";
$url_chaudiere = "http://192.168.1.53";
$port_json = "4321";
$password_json = "RwKS";
$param = "all?";
$url = $url_chaudiere.":".$port_json."/".$password_json."/";

$url = $url.$param;


// récupération du fichier et conversion en UTF8
$curl = curl_init();
curl_setopt($curl, CURLOPT_URL, $url);
curl_setopt($curl, CURLOPT_RETURNTRANSFER, 1);
$output = utf8_encode(curl_exec($curl));
curl_close($curl);

// insertion dans un tableau
$tableau_boiler = json_decode($output,true);



//pe1
		$L_type="0:PE|1:PES|2:PEK|3:PESK|4:SMART V1|5:SMART V2|6:PE CONDENS|7:SMART XS|8:SMART V3|9:COMPACT|10:AIR";
		$data["pe1_name"] = substr(explode("|",$L_type)[$tableau_boiler['pe1']['L_type']['val']],2);
		
		
		$data["pe1_temp"] = $tableau_boiler['pe1']['L_temp_act']['val']/10;
		$data["pe1_temp"] = $data["pe1_temp"].' '.$tableau_boiler['pe1']['L_temp_act']['unit'];
		
		$data["pe1_state_text"] = 'Etat de la chaudière : '.str_replace('|',' | ',$tableau_boiler['pe1']['L_statetext']);
		$data["pe1_runtime"] = 'temps de fonctionnement : '.$tableau_boiler['pe1']['L_runtime']['val']." ".$tableau_boiler['pe1']['L_runtime']['unit'];
		
		$data["pe1_avg_runtime"] = 'temps de fonctionnement moyen : '.$tableau_boiler['pe1']['L_avg_runtime']['val']." ".$tableau_boiler['pe1']['L_avg_runtime']['unit'];
		$data["pe1_pellets_today"] = 'consommation du jour : '.$tableau_boiler['pe1']['storage_fill_today']['val']." ".$tableau_boiler['pe1']['storage_fill_today']['unit'];
		$data["pe1_pellets_yesterday"] = 'consommation de la veille : '.$tableau_boiler['pe1']['storage_fill_yesterday']['val']." ".$data["pe1_pellets_yesterday"] = $tableau_boiler['pe1']['storage_fill_yesterday']['unit'];
		$data["pe1_pellets"] = 'stock restant dans le silo : '.$tableau_boiler['pe1']['L_storage_fill']['val']." ".$data["pe1_pellets"] = $tableau_boiler['pe1']['L_storage_fill']['unit'];
		$data["pe1_pellets_min"] = 'stockage min : '.$tableau_boiler['pe1']['L_storage_min']['val']." ".$data["pe1_pellets_min"] = $tableau_boiler['pe1']['L_storage_min']['unit'];
		$data["pe1_pellets_max"] = 'stockage max : '.$tableau_boiler['pe1']['L_storage_max']['val']." ".$data["pe1_pellets_max"] = $tableau_boiler['pe1']['L_storage_max']['unit'];
		$data["pe1_pellets_popper"] = 'stock restant dans la chaudière : '.$tableau_boiler['pe1']['L_storage_popper']['val']." ".$data["pe1_pellets_popper"] = $tableau_boiler['pe1']['L_storage_popper']['unit'];
		$data["pe1_modulation"] = 'modulation : '.$tableau_boiler['pe1']['L_modulation']['val']." ".$data["pe1_modulation"] = $tableau_boiler['pe1']['L_modulation']['unit'];
		$data["pe1_ventilation"] = 'ventilation : '.$tableau_boiler['pe1']['L_currentairflow']['val']." ".$data["pe1_ventilation"] = $tableau_boiler['pe1']['L_currentairflow']['unit'];		
        $data["pe1_fluegas"] = 'gaz de combustion : '.$tableau_boiler['pe1']['L_fluegas']['val']." ".$data["pe1_fluegas"] = $tableau_boiler['pe1']['L_fluegas']['unit'];		
        $data["pe1_uw_speed"] = 'vitesse de la pompe de circulation : '.$tableau_boiler['pe1']['L_uw_speed']['val']." ".$data["pe1_uw_speed"] = $tableau_boiler['pe1']['L_uw_speed']['unit'];		
        $data["pe1_depression"] = 'dépression : '.$tableau_boiler['pe1']['L_lowpressure']['val']." ".$data["pe1_lowpressure"] = $tableau_boiler['pe1']['L_lowpressure']['unit'];



	//hk1
		$data["hk1_name"] = $tableau_boiler['hk1']['name']['val'];
		
		$data["hk1_temp"] = $tableau_boiler['hk1']['L_roomtemp_act']['val']/10;
		$data["hk1_temp"] = $data["hk1_temp"].' '.$tableau_boiler['hk1']['L_roomtemp_act']['unit'];
		
		$data["hk1_consigne"] = $tableau_boiler['hk1']['L_roomtemp_set']['val']/10 ;
		$data["hk1_consigne"] = $data["hk1_consigne"].' '.$tableau_boiler['hk1']['L_roomtemp_set']['unit'] ;
		
		$data["hk1_state_text"] = str_replace('|',' | ',$tableau_boiler['hk1']['L_statetext']);
		
		$data["hk1_pump"] = $tableau_boiler['hk1']['L_pump']['val'];
		
		//hk2
		$data["hk2_name"] = $tableau_boiler['hk2']['name']['val'];
		
		$data["hk2_temp"] = $tableau_boiler['hk2']['L_roomtemp_act']['val']/10;
		$data["hk2_temp"] = $data["hk2_temp"].' '.$tableau_boiler['hk2']['L_roomtemp_act']['unit'];
		
		$data["hk2_consigne"] = $tableau_boiler['hk2']['L_roomtemp_set']['val']/10;
		$data["hk2_consigne"] = $data["hk2_consigne"].' '.$tableau_boiler['hk2']['L_roomtemp_set']['unit'] ;
		
		$data["hk2_state_text"] = str_replace('|',' | ',$tableau_boiler['hk2']['L_statetext']);
		
		$data["hk2_pump"] = $tableau_boiler['hk2']['L_pump']['val'];
		
		//ecs
		$data["ww1_name"] = $tableau_boiler['ww1']['name']['val'];
		$data["ww1_temp"] = $tableau_boiler['ww1']['L_ontemp_act']['val']/10;
		$data["ww1_temp"] = $data["ww1_temp"].' '.$tableau_boiler['ww1']['L_ontemp_act']['unit'];
		$data["ww1_state_text"] = str_replace('|',' | ',$tableau_boiler['ww1']['L_statetext']);
		$data["ww1_pump"] = $tableau_boiler['ww1']['L_pump']['val'];
		
		$data["ww1_consigne"] = $tableau_boiler['ww1']['L_temp_set']['val']/10 ;
		$data["ww1_consigne"] = $data["ww1_consigne"].' '.$tableau_boiler['ww1']['L_temp_set']['unit'] ;
		
		
	
		//météo
		$data["meteo"] = explode("|",$tableau_boiler['forecast']['L_w_0']['val']);
		//durée du jour
		$date_D_jour = DateTimeImmutable::createFromFormat('H:i',$data["meteo"] [7]);
		$date_F_jour = DateTimeImmutable::createFromFormat('H:i',$data["meteo"] [8]);
		$duree_jour = $date_D_jour->diff($date_F_jour);
		$duree_jour = ($duree_jour->format('%H:%I'));
		$data["meteo"][9]=$duree_jour;




return $data;
}


	
	

	



 }
 



